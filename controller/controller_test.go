package controller

import (
	"k8s.io/client-go/tools/record"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/bluehoodie/crypt-controller/pkg/apis/crypt/v1alpha1"
	cryptfake "github.com/bluehoodie/crypt-controller/pkg/client/clientset/versioned/fake"
	cryptinformers "github.com/bluehoodie/crypt-controller/pkg/client/informers/externalversions"
	"github.com/bluehoodie/crypt-controller/pkg/store"
	"github.com/bluehoodie/crypt-controller/pkg/store/memory"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type cryptOpts struct {
	name      string
	namespace string

	secrets          []v1alpha1.SecretDefinition
	targetNamespaces []string
}

func newCrypt(opts *cryptOpts) *v1alpha1.Crypt {
	return &v1alpha1.Crypt{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.name,
			Namespace: opts.namespace,
		},
		Spec: v1alpha1.CryptSpec{
			Secrets:    opts.secrets,
			Namespaces: opts.targetNamespaces,
		},
	}
}

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

type fixture struct {
	t *testing.T

	controller    *Controller
	k8sInformer   kubeinformers.SharedInformerFactory
	cryptInformer cryptinformers.SharedInformerFactory

	kubeclient  *kubefake.Clientset
	cryptclient *cryptfake.Clientset

	cryptObjects []runtime.Object
	kubeObjects  []runtime.Object

	namespaceLister []*v1.Namespace
	cryptLister     []*v1alpha1.Crypt

	kubeActions  []core.Action
	cryptActions []core.Action

	store store.Store
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{
		t: t,
	}

	testStoreMap := make(map[string]store.Object)
	testStoreMap["test/foo"] = store.Object(map[string][]byte{"foo": []byte("fooSecret")})
	testStoreMap["test/bar"] = store.Object(map[string][]byte{"bar": []byte("barSecret")})
	store, _ := memory.New(testStoreMap)
	f.store = store

	f.kubeObjects = []runtime.Object{}
	f.cryptObjects = []runtime.Object{}

	f.namespaceLister = []*v1.Namespace{}
	f.cryptLister = []*v1alpha1.Crypt{}

	f.initController()

	return f
}

func (f *fixture) initController() {
	f.cryptclient = cryptfake.NewSimpleClientset(f.cryptObjects...)
	f.kubeclient = kubefake.NewSimpleClientset(f.cryptObjects...)

	f.cryptInformer = cryptinformers.NewSharedInformerFactory(f.cryptclient, noResyncPeriodFunc())
	f.k8sInformer = kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	f.controller = New(f.kubeclient, f.cryptclient,
		f.k8sInformer.Core().V1().Namespaces(),
		f.k8sInformer.Core().V1().Secrets(),
		f.cryptInformer.Core().V1alpha1().Crypts(),
		f.store,
		WithEventRecorder(record.NewFakeRecorder(10)),
	)
	f.controller.cryptInformerSynced = alwaysReady
	f.controller.namespaceInformerSynced = alwaysReady
	f.controller.secretInformerSynced = alwaysReady
}

func (f *fixture) initControllerLists() {
	for _, o := range f.cryptLister {
		f.cryptInformer.Core().V1alpha1().Crypts().Informer().GetIndexer().Add(o)
	}

	for _, o := range f.namespaceLister {
		f.k8sInformer.Core().V1().Namespaces().Informer().GetIndexer().Add(o)
	}
}

func (f *fixture) run(cryptName string) {
	f.runController(cryptName, false)
}

func (f *fixture) runExpectError(cryptName string) {
	f.runController(cryptName, true)
}

func (f *fixture) runController(cryptName string, expectError bool) {
	f.initControllerLists()

	//start informers
	stop := make(chan struct{})
	defer close(stop)
	f.k8sInformer.Start(stop)
	f.cryptInformer.Start(stop)

	err := f.controller.syncHandler(cryptName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing crypt: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing crypt, got nil")
	}

	kubeClientActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range kubeClientActions {
		if len(f.kubeActions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(kubeClientActions)-len(f.kubeActions), kubeClientActions[i:])
			break
		}

		expectedAction := f.kubeActions[i]
		checkAction(expectedAction, action, f.t)
	}

	cryptClientActions := filterInformerActions(f.cryptclient.Actions())
	for i, action := range cryptClientActions {
		if len(f.cryptActions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(cryptClientActions)-len(f.cryptActions), cryptClientActions[i:])
			break
		}

		expectedAction := f.cryptActions[i]
		checkAction(expectedAction, action, f.t)
	}
}

func (f *fixture) expectCreateSecretAction(secret *v1.Secret) {
	f.kubeActions = append(f.kubeActions, core.NewCreateAction(schema.GroupVersionResource{Resource: "secrets"}, secret.Namespace, secret))
}

func filterInformerActions(actions []core.Action) []core.Action {
	ret := make([]core.Action, 0, 0)
	for _, action := range actions {
		if action.Matches("list", "crypts") ||
			action.Matches("watch", "crypts") ||
			action.Matches("list", "namespaces") ||
			action.Matches("watch", "namespaces") ||
			action.Matches("update", "namespaces") ||
			action.Matches("list", "secrets") ||
			action.Matches("update", "secrets") ||
			action.Matches("watch", "secrets") {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch))
		}
	}
}

func getKey(crypt *v1alpha1.Crypt, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(crypt)
	if err != nil {
		t.Errorf("Unexpected error getting key for crypt %v: %v", crypt.Name, err)
		return ""
	}
	return key
}

func TestSecretsCreated(t *testing.T) {
	f := newFixture(t)

	secretDefinitions := []v1alpha1.SecretDefinition{
		{
			Name: "test-foo-secret",
			Key:  "test/foo",
		},
		{
			Name: "test-bar-secret",
			Key:  "test/bar",
		},
	}

	namespaceStrings := []string{"test-ns1", "test-ns2"}
	var namespaces []*v1.Namespace

	for _, nsName := range namespaceStrings {
		namespaces = append(namespaces, newNamespace(nsName))
	}

	crypt := newCrypt(&cryptOpts{
		name:             "test-crypt",
		namespace:        "default",
		targetNamespaces: namespaceStrings,
		secrets:          secretDefinitions,
	})

	f.cryptLister = append(f.cryptLister, crypt)
	f.cryptObjects = append(f.cryptObjects, crypt)

	for _, ns := range namespaces {
		f.namespaceLister = append(f.namespaceLister, ns)
		f.kubeObjects = append(f.kubeObjects, ns)
	}

	for _, secretdef := range secretDefinitions {
		obj, _ := f.store.Get(secretdef.Key)
		for _, namespace := range namespaceStrings {
			expectedSecret := newSecret(obj.GetData(), secretdef, crypt, namespace)
			f.expectCreateSecretAction(expectedSecret)
		}
	}

	f.run(getKey(crypt, t))
}

func TestSecretsCreatedNamespacePatterns(t *testing.T) {
	f := newFixture(t)

	secretDefinitions := []v1alpha1.SecretDefinition{
		{
			Name: "test-foo-secret",
			Key:  "test/foo",
		},
		{
			Name: "test-bar-secret",
			Key:  "test/bar",
		},
	}

	namespacePatterns := []string{"test-ns*"}
	namespaceStrings := []string{"test-ns1", "test-ns2"}
	var namespaces []*v1.Namespace

	for _, nsName := range namespaceStrings {
		namespaces = append(namespaces, newNamespace(nsName))
	}

	crypt := newCrypt(&cryptOpts{
		name:             "test-crypt",
		namespace:        "default",
		targetNamespaces: namespacePatterns,
		secrets:          secretDefinitions,
	})

	f.cryptLister = append(f.cryptLister, crypt)
	f.cryptObjects = append(f.cryptObjects, crypt)

	for _, ns := range namespaces {
		f.namespaceLister = append(f.namespaceLister, ns)
		f.kubeObjects = append(f.kubeObjects, ns)
	}

	for _, secretdef := range secretDefinitions {
		obj, _ := f.store.Get(secretdef.Key)
		for _, namespace := range namespaceStrings {
			expectedSecret := newSecret(obj.GetData(), secretdef, crypt, namespace)
			f.expectCreateSecretAction(expectedSecret)
		}
	}

	f.run(getKey(crypt, t))
}
