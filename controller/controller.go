package controller

import (
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	log "k8s.io/klog"

	"github.com/bluehoodie/crypt-controller/pkg/apis/crypt/v1alpha1"
	clientset "github.com/bluehoodie/crypt-controller/pkg/client/clientset/versioned"
	cryptscheme "github.com/bluehoodie/crypt-controller/pkg/client/clientset/versioned/scheme"
	informers "github.com/bluehoodie/crypt-controller/pkg/client/informers/externalversions/crypt/v1alpha1"
	listers "github.com/bluehoodie/crypt-controller/pkg/client/listers/crypt/v1alpha1"
	"github.com/bluehoodie/crypt-controller/pkg/store"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Crypt is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Crypt is synced successfully
	MessageResourceSynced = "Crypt synced successfully"
)

const controllerAgentName = "crypt-controller"

type Controller struct {
	queue workqueue.RateLimitingInterface

	kubeClientset  kubernetes.Interface
	cryptClientset clientset.Interface

	namespaceInformerSynced cache.InformerSynced
	namespaceLister         v1listers.NamespaceLister
	secretInformerSynced    cache.InformerSynced
	secretLister            v1listers.SecretLister
	cryptInformerSynced     cache.InformerSynced
	cryptLister             listers.CryptLister

	recorder record.EventRecorder

	store store.Store
}

func New(
	kubeClientset kubernetes.Interface,
	cryptClientset clientset.Interface,
	namespaceInformer coreinformers.NamespaceInformer,
	secreteInformer coreinformers.SecretInformer,
	cryptInformer informers.CryptInformer,
	store store.Store,
) *Controller {

	utilruntime.Must(cryptscheme.AddToScheme(scheme.Scheme))

	log.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	c := Controller{
		kubeClientset:  kubeClientset,
		cryptClientset: cryptClientset,

		namespaceInformerSynced: namespaceInformer.Informer().HasSynced,
		namespaceLister:         namespaceInformer.Lister(),
		secretInformerSynced:    secreteInformer.Informer().HasSynced,
		secretLister:            secreteInformer.Lister(),
		cryptInformerSynced:     cryptInformer.Informer().HasSynced,
		cryptLister:             cryptInformer.Lister(),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "crypt-controller"),

		recorder: recorder,

		store: store,
	}

	cryptInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueCrypt(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			c.enqueueCrypt(new)
		},
		DeleteFunc: func(obj interface{}) {
			c.deleteCrypt(obj)
		},
	})

	secreteInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			c.handleSecretDelete(obj)
		},
	})

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.handleNamespaceAdd(obj)
		},
	})

	return &c
}

func (c *Controller) Run(workers int, stopChan <-chan struct{}) error {
	defer utilruntime.HandleCrash() //soon to be deprecated?
	defer c.queue.ShutDown()

	log.Info("starting Crypt controller")

	timeoutChan := make(chan struct{})
	go func() {
		defer close(timeoutChan)
		select {
		case <-stopChan:
			return
		case <-time.After(5 * time.Minute):
			return
		}
	}()

	ok := cache.WaitForCacheSync(timeoutChan, c.namespaceInformerSynced, c.secretInformerSynced, c.cryptInformerSynced)
	if !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Info("starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopChan)
	}

	log.Info("started workers")
	<-stopChan
	log.Info("stopping workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in queue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.queue.Forget(obj)
		log.Infof("successfully synced %s", key)

		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) enqueueCrypt(obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	crypt, err := c.cryptLister.Crypts(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("crypt %s in work queue no longer exists", key))
			return nil
		}
	}

	// create secrets in the appropriate namespaces
	for _, key := range crypt.Spec.Keys {
		for _, namespacePattern := range crypt.Spec.Namespaces {
			for _, ns := range c.findNamespaceMatches(namespacePattern) {
				err := c.createSecret(key, crypt, ns)
				if err != nil {
					log.Infof("could not create secret for key %s in namespace %s: %v", key, namespace, err)
				}
			}
		}
	}

	_, err = c.cryptClientset.CoreV1alpha1().Crypts(crypt.Namespace).Update(crypt.DeepCopy())
	if err != nil {
		return err
	}

	c.recorder.Event(crypt, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) deleteCrypt(obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	// delete every secret owned by this crypt
	crypt := obj.(*v1alpha1.Crypt)
	for _, key := range crypt.Spec.Keys {
		for _, namespacePattern := range crypt.Spec.Namespaces {
			for _, namespace := range c.findNamespaceMatches(namespacePattern) {
				if err := c.deleteSecret(key, namespace); err != nil {
					utilruntime.HandleError(err)
				}
			}
		}
	}

	c.queue.Forget(key)
}

func (c *Controller) createSecret(key string, crypt *v1alpha1.Crypt, namespace string) error {
	obj, err := c.store.Get(key)
	if err != nil {
		log.Errorf("could not get value from store: %v", err)
		return err
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetName(),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(crypt, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Crypt",
				}),
			},
		},
		Type: corev1.SecretType(obj.GetSecretType()),
		Data: obj.GetData(),
	}
	_, err = c.kubeClientset.CoreV1().Secrets(namespace).Create(secret)
	return err
}

func (c *Controller) deleteSecret(key string, namespace string) error {
	obj, err := c.store.Get(key)
	if err != nil {
		return err
	}

	return c.kubeClientset.CoreV1().Secrets(namespace).Delete(obj.Name, metav1.NewDeleteOptions(3))
}

func (c *Controller) handleNamespaceAdd(obj interface{}) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return
	}

	nsList, err := c.namespaceLister.List(labels.NewSelector())
	if err != nil {
		return
	}

	for _, ns := range nsList {
		cryptList, err := c.cryptLister.Crypts(ns.Name).List(labels.NewSelector())
		if err != nil {
			continue
		}

	Out:
		for _, crypt := range cryptList {
			for _, namespacePattern := range crypt.Spec.Namespaces {
				match, _ := regexp.MatchString(namespacePattern, namespace.Name)
				if match {
					c.enqueueCrypt(crypt)
					break Out
				}
			}
		}
	}
}

func (c *Controller) handleSecretDelete(obj interface{}) {
	// check to see if this secret belonged to an active crypt. if yes, then re-create the secret
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Errorf("Couldn't get object from tombstone %+v", obj)
			return
		}
		secret, ok = tombstone.Obj.(*corev1.Secret)
		if !ok {
			log.Errorf("Tombstone contained object that is not a secret %+v", obj)
			return
		}
	}

	if ownerRef := metav1.GetControllerOf(secret); ownerRef != nil {
		// If this object is not owned by a Crypt, we should not do anything more with it.
		if ownerRef.Kind != "Crypt" {
			return
		}

		//find the right crypt associated with this secret across all namespaces
		var crypt *v1alpha1.Crypt

		crypts, _ := c.cryptClientset.CoreV1alpha1().Crypts("").List(metav1.ListOptions{})
		found := false
		namespacesChecked := make(map[string]struct{})
		for _, ci := range crypts.Items {
			namespace := ci.Namespace
			if _, ok := namespacesChecked[namespace]; ok {
				continue
			}
			namespacesChecked[namespace] = struct{}{}

			crypt, _ = c.cryptLister.Crypts(namespace).Get(ownerRef.Name)
			if crypt != nil {
				found = true
				break
			}
		}

		if !found {
			log.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", secret.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueCrypt(crypt)
		return
	}
}

func (c *Controller) findNamespaceMatches(namespacePattern string) []string {
	var result []string

	namespaces, _ := c.kubeClientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	for _, ns := range namespaces.Items {
		match, _ := regexp.MatchString(namespacePattern, ns.Name)
		if match {
			result = append(result, ns.Name)
		}
	}

	return result
}
