package controller

import (
	"reflect"
	"testing"
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	coretest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	cryptcontroller "github.com/bluehoodie/crypt-controller/pkg/apis/crypt/v1alpha1"
	cryptfake "github.com/bluehoodie/crypt-controller/pkg/client/clientset/versioned/fake"
	cryptinformers "github.com/bluehoodie/crypt-controller/pkg/client/informers/externalversions"
	"github.com/bluehoodie/crypt-controller/pkg/store/memory"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client     *cryptfake.Clientset
	kubeclient *k8sfake.Clientset

	// Objects to put in the store.
	cryptLister     []*cryptcontroller.Crypt
	namespaceLister []*core.Namespace
	secretLister    []*core.Secret

	// Actions expected to happen on the client.
	kubeActions []coretest.Action
	actions     []coretest.Action

	// Objects from here preloaded into NewSimpleFake.
	kubeObjects []runtime.Object
	objects     []runtime.Object

	store *memory.Store
}

func newFixture(t *testing.T) *fixture {
	f := fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeObjects = []runtime.Object{}

	return &f
}

func newTestCrypt(name string, specNamespaces []string, specKeys []string) *cryptcontroller.Crypt {
	crypt := cryptcontroller.Crypt{
		TypeMeta: metav1.TypeMeta{
			APIVersion: cryptcontroller.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cryptcontroller.CryptSpec{
			Namespaces: specNamespaces,
			Keys:       specKeys,
		},
	}

	return &crypt
}

func newTestNamespace(name string) *core.Namespace {
	namespace := core.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return &namespace
}

func newTestSecret(name string) *core.Secret {
	secret := core.Secret{}

	return &secret
}

func (f *fixture) newController() (*Controller, cryptinformers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = cryptfake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	cryptInformerFactory := cryptinformers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	c := New(f.kubeclient, f.client,
		kubeInformerFactory.Core().V1().Namespaces(),
		kubeInformerFactory.Core().V1().Secrets(),
		cryptInformerFactory.Core().V1alpha1().Crypts(), f.store)

	c.cryptInformerSynced = alwaysReady
	c.secretInformerSynced = alwaysReady
	c.namespaceInformerSynced = alwaysReady

	c.recorder = &record.FakeRecorder{}

	for _, c := range f.cryptLister {
		cryptInformerFactory.Core().V1alpha1().Crypts().Informer().GetIndexer().Add(c)
	}

	for _, n := range f.namespaceLister {
		kubeInformerFactory.Core().V1().Namespaces().Informer().GetIndexer().Add(n)
	}

	for _, s := range f.secretLister {
		kubeInformerFactory.Core().V1().Secrets().Informer().GetIndexer().Add(s)
	}

	return c, cryptInformerFactory, kubeInformerFactory
}

func (f *fixture) run(cryptName string) {
	f.runController(cryptName, true, false)
}

func (f *fixture) runExpectError(cryptName string) {
	f.runController(cryptName, true, true)
}

func (f *fixture) runController(cryptName string, startInformers bool, expectError bool) {
	c, cryptinformer, kubeinformer := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		cryptinformer.Start(stopCh)
		kubeinformer.Start(stopCh)
	}

	err := c.syncHandler(cryptName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeActions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeActions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeActions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeActions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeActions)-len(k8sActions), f.kubeActions[len(k8sActions):])
	}
}

func (f *fixture) expectUpdateFooStatusAction(foo *cryptcontroller.Crypt) {
	action := coretest.NewUpdateAction(schema.GroupVersionResource{Resource: "crypts"}, foo.Namespace, foo)
	f.actions = append(f.actions, action)
}

func TestDoNothing(t *testing.T) {
	f := newFixture(t)
	foo := newTestCrypt("test", []string{}, []string{})

	f.cryptLister = append(f.cryptLister, foo)
	f.objects = append(f.objects, foo)

	f.expectUpdateFooStatusAction(foo)
	f.run(getKey(foo, t))
}

func getKey(foo *cryptcontroller.Crypt, t *testing.T) string {
	key, err := cache.MetaNamespaceKeyFunc(foo)
	if err != nil {
		t.Errorf("Unexpected error getting key for foo %v: %v", foo.Name, err)
		return ""
	}
	return key
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []coretest.Action) []coretest.Action {
	var ret []coretest.Action
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "crypts") ||
				action.Matches("watch", "crypts") ||
				action.Matches("list", "namespaces") ||
				action.Matches("watch", "namespaces") ||
				action.Matches("list", "secrets") ||
				action.Matches("watch", "secrets")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual coretest.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case coretest.CreateAction:
		e, _ := expected.(coretest.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case coretest.UpdateAction:
		e, _ := expected.(coretest.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case coretest.DeleteAction:
		e, _ := expected.(coretest.DeleteAction)
		expName := e.GetName()
		name := a.GetName()

		if !reflect.DeepEqual(expName, name) {
			t.Errorf("Action %s %s has wrong name\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expName, name))
		}
	case coretest.PatchAction:
		e, _ := expected.(coretest.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch))
		}
	}
}
