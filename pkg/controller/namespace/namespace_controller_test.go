package namespace

import (
	"context"
	"github.com/openshift/dedicated-admin-operator/pkg/dedicatedadmin"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	realclient "sigs.k8s.io/controller-runtime/pkg/client"
	client "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func reset(ctx context.Context, r *ReconcileNamespace) {
	blacklistedProjects = make(map[string]bool)
	// try to delete test objects
	r.client.Delete(ctx, makeConfig())
	r.client.Delete(ctx, makeNamespace("test"))
}

func newTestReconciler() *ReconcileNamespace {
	return &ReconcileNamespace{
		client: client.NewFakeClient(),
		scheme: nil,
	}
}

func TestMissingConfigMap(t *testing.T) {
	ctx := context.TODO()

	reconciler := newTestReconciler()
	defer reset(ctx, reconciler)

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	}
	res, err := reconciler.Reconcile(request)
	if !res.Requeue {
		t.Error("Expected to be told to requeue because there's no configmap, but we weren't")
	}
	if err == nil {
		t.Error("Expected an error because there's no configmap, but didn't get one")
	}
}

func TestBlockedNamespace(t *testing.T) {

	ctx := context.TODO()
	reconciler := newTestReconciler()
	defer reset(ctx, reconciler)

	reconciler.client.Create(ctx, makeConfig())

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "kube-system",
			Namespace: "",
		},
	}

	res, err := reconciler.Reconcile(request)

	if res.Requeue {
		t.Error("Didn't expect to requeue")
	}
	if err != nil {
		t.Errorf("Got an unexpected error: %s", err)
	}
}

func TestUnBlockedNamespace(t *testing.T) {

	ctx := context.TODO()
	reconciler := newTestReconciler()
	defer reset(ctx, reconciler)
	cerr := reconciler.client.Create(ctx, makeConfig())
	if cerr != nil {
		t.Error("Couldn't create the required configmap for the test")
	}
	nerr := reconciler.client.Create(ctx, makeNamespace("test"))
	if nerr != nil {
		t.Error("Couldn't create the required namespace for the test")
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test",
			Namespace: "",
		},
	}

	res, err := reconciler.Reconcile(request)

	if res.Requeue {
		t.Error("Didn't expect to requeue")
	}
	if err != nil {
		t.Errorf("Got an unexpected error: %s", err)
	}
	// Validate RoleBindings were added

	list := rbacv1.RoleBindingList{}
	opts := realclient.ListOptions{Namespace: request.Name}

	err = reconciler.client.List(ctx, &opts, &list)
	if err != nil {
		t.Errorf("Error while trying to list RBAC entries: %s", err)
	}
	// we have some RoleBindings in dedicatedadmin.Rolebindings, so let's make sure we have them here, too
	seen := make(map[string]bool)
	for _, rb := range dedicatedadmin.Rolebindings {
		seen[rb.ObjectMeta.Name] = false
	}
	for _, rb := range list.Items {
		seen[rb.ObjectMeta.Name] = true
	}
	for rb_name, s := range seen {
		if !s {
			t.Errorf("Expected to have RoleBinding created with the name %s, but didn't see one", rb_name)
		}
	}
}

func makeNamespace(ns string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}
}

func makeConfig() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dedicated-admin-operator-config",
			Namespace: "openshift-dedicated-admin",
		},
		Data: map[string]string{
			"project_blacklist": "^kube-.*,^openshift-.*,^logging$,^default$,^openshift$",
		},
	}
}
