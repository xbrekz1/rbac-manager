package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rbacmanagerv1alpha1 "github.com/xbrekz1/rbac-manager/api/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "templates")},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = rbacmanagerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&AccessGrantReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("AccessGrant Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating an AccessGrant with predefined role", func() {
		It("Should create ServiceAccount, Role, and RoleBinding", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-basic",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create target namespace for RBAC
			targetNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns-basic",
				},
			}
			Expect(k8sClient.Create(ctx, targetNs)).Should(Succeed())

			// Create AccessGrant
			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-basic",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role:               rbacmanagerv1alpha1.RoleDeveloper,
					Namespaces:         []string{targetNs.Name},
					ServiceAccountName: "test-sa-basic",
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// Check ServiceAccount
			saKey := types.NamespacedName{Name: "test-sa-basic", Namespace: testNs.Name}
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Check Role in target namespace
			roleKey := types.NamespacedName{Name: "rbac-test-grant-basic", Namespace: targetNs.Name}
			role := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleKey, role)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Check RoleBinding
			rbKey := types.NamespacedName{Name: "rbac-test-grant-basic", Namespace: targetNs.Name}
			rb := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rbKey, rb)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Verify RoleBinding references
			Expect(rb.RoleRef.Name).Should(Equal("rbac-test-grant-basic"))
			Expect(rb.Subjects).Should(HaveLen(1))
			Expect(rb.Subjects[0].Name).Should(Equal("test-sa-basic"))
			Expect(rb.Subjects[0].Namespace).Should(Equal(testNs.Name))

			// Check AccessGrant status
			agKey := types.NamespacedName{Name: ag.Name, Namespace: ag.Namespace}
			Eventually(func() rbacmanagerv1alpha1.Phase {
				err := k8sClient.Get(ctx, agKey, ag)
				if err != nil {
					return ""
				}
				return ag.Status.Phase
			}, timeout, interval).Should(Equal(rbacmanagerv1alpha1.PhaseActive))
		})
	})

	Context("When creating an AccessGrant with ClusterWide mode", func() {
		It("Should create ClusterRole and ClusterRoleBinding", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-cluster",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create AccessGrant
			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-cluster",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role:               rbacmanagerv1alpha1.RoleViewer,
					ClusterWide:        true,
					ServiceAccountName: "test-sa-cluster",
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// Check ServiceAccount
			saKey := types.NamespacedName{Name: "test-sa-cluster", Namespace: testNs.Name}
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Check ClusterRole
			crKey := types.NamespacedName{Name: "rbac-test-grant-cluster"}
			cr := &rbacv1.ClusterRole{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, crKey, cr)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Check ClusterRoleBinding
			crbKey := types.NamespacedName{Name: "rbac-test-grant-cluster"}
			crb := &rbacv1.ClusterRoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, crbKey, crb)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Verify ClusterRoleBinding references
			Expect(crb.RoleRef.Name).Should(Equal("rbac-test-grant-cluster"))
			Expect(crb.Subjects).Should(HaveLen(1))
			Expect(crb.Subjects[0].Name).Should(Equal("test-sa-cluster"))
		})
	})

	Context("When creating an AccessGrant with custom rules", func() {
		It("Should create resources with custom policy rules", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-custom",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create target namespace
			targetNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns-custom",
				},
			}
			Expect(k8sClient.Create(ctx, targetNs)).Should(Succeed())

			// Create AccessGrant with custom rules
			customRules := []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services"},
					Verbs:     []string{"get", "list"},
				},
			}

			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-custom",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					CustomRules:        customRules,
					Namespaces:         []string{targetNs.Name},
					ServiceAccountName: "test-sa-custom",
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// Check Role has custom rules
			roleKey := types.NamespacedName{Name: "rbac-test-grant-custom", Namespace: targetNs.Name}
			role := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleKey, role)
				if err != nil {
					return false
				}
				return len(role.Rules) == 1 &&
					len(role.Rules[0].Resources) == 2 &&
					role.Rules[0].Resources[0] == "pods"
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When deleting an AccessGrant", func() {
		It("Should cleanup all managed resources", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-delete",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create target namespace
			targetNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns-delete",
				},
			}
			Expect(k8sClient.Create(ctx, targetNs)).Should(Succeed())

			// Create AccessGrant
			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-delete",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role:               rbacmanagerv1alpha1.RoleDeveloper,
					Namespaces:         []string{targetNs.Name},
					ServiceAccountName: "test-sa-delete",
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// Wait for resources to be created
			saKey := types.NamespacedName{Name: "test-sa-delete", Namespace: testNs.Name}
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			roleKey := types.NamespacedName{Name: "rbac-test-grant-delete", Namespace: targetNs.Name}
			role := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleKey, role)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Delete AccessGrant
			Expect(k8sClient.Delete(ctx, ag)).Should(Succeed())

			// Verify resources are deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleKey, role)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When target namespace does not exist", func() {
		It("Should skip the namespace and continue", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-missing",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create AccessGrant referencing non-existent namespace
			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-missing-ns",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role:               rbacmanagerv1alpha1.RoleViewer,
					Namespaces:         []string{"non-existent-namespace"},
					ServiceAccountName: "test-sa-missing",
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// ServiceAccount should still be created
			saKey := types.NamespacedName{Name: "test-sa-missing", Namespace: testNs.Name}
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Status should be Active but with empty namespaces
			agKey := types.NamespacedName{Name: ag.Name, Namespace: ag.Namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, agKey, ag)
				if err != nil {
					return false
				}
				return ag.Status.Phase == rbacmanagerv1alpha1.PhaseActive &&
					len(ag.Status.Namespaces) == 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When AccessGrant has labels and annotations", func() {
		It("Should propagate labels and annotations to managed resources", func() {
			ctx := context.Background()

			// Create a test namespace
			testNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-labels",
				},
			}
			Expect(k8sClient.Create(ctx, testNs)).Should(Succeed())

			// Create target namespace
			targetNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns-labels",
				},
			}
			Expect(k8sClient.Create(ctx, targetNs)).Should(Succeed())

			// Create AccessGrant with labels and annotations
			ag := &rbacmanagerv1alpha1.AccessGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant-labels",
					Namespace: testNs.Name,
				},
				Spec: rbacmanagerv1alpha1.AccessGrantSpec{
					Role:               rbacmanagerv1alpha1.RoleReader,
					Namespaces:         []string{targetNs.Name},
					ServiceAccountName: "test-sa-labels",
					Labels: map[string]string{
						"team":        "platform",
						"environment": "test",
					},
					Annotations: map[string]string{
						"owner":      "admin@example.com",
						"expires-at": "2027-12-31",
					},
				},
			}
			Expect(k8sClient.Create(ctx, ag)).Should(Succeed())

			// Check ServiceAccount has labels and annotations
			saKey := types.NamespacedName{Name: "test-sa-labels", Namespace: testNs.Name}
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saKey, sa)
				if err != nil {
					return false
				}
				return sa.Labels["team"] == "platform" &&
					sa.Labels[managedByLabel] == managerValue &&
					sa.Annotations["owner"] == "admin@example.com"
			}, timeout, interval).Should(BeTrue())

			// Check Role has labels
			roleKey := types.NamespacedName{Name: "rbac-test-grant-labels", Namespace: targetNs.Name}
			role := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleKey, role)
				if err != nil {
					return false
				}
				return role.Labels["team"] == "platform" &&
					role.Labels[accessGrantLabel] == "test-grant-labels"
			}, timeout, interval).Should(BeTrue())
		})
	})
})
