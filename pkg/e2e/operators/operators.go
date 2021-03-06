package operators

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	kerror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/openshift/osde2e/pkg/common/config"
	"github.com/openshift/osde2e/pkg/common/helper"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
)

func checkClusterServiceVersion(h *helper.H, namespace, name string) {
	// Check that the operator clusterServiceVersion exists
	ginkgo.Context("clusterServiceVersion", func() {
		ginkgo.It("should exist", func() {
			csvs, err := pollCsvList(h, namespace, name)
			Expect(err).ToNot(HaveOccurred(), "failed fetching the clusterServiceVersions")
			Expect(csvs).NotTo(BeNil())
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func checkConfigMapLockfile(h *helper.H, namespace, operatorLockFile string) {
	// Check that the operator configmap has been deployed
	ginkgo.Context("configmaps", func() {
		ginkgo.It("should exist", func() {
			// Wait for lockfile to signal operator is active
			err := pollLockFile(h, namespace, operatorLockFile)
			Expect(err).ToNot(HaveOccurred(), "failed fetching the configMap lockfile")
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func checkDeployment(h *helper.H, namespace string, name string, defaultDesiredReplicas int32) {
	// Check that the operator deployment exists in the operator namespace
	ginkgo.Context("deployment", func() {
		ginkgo.It("should exist", func() {
			deployment, err := pollDeployment(h, namespace, name)
			Expect(err).ToNot(HaveOccurred(), "failed fetching deployment")
			Expect(deployment).NotTo(BeNil(), "deployment is nil")
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
		ginkgo.It("should have all desired replicas ready", func() {
			deployment, err := pollDeployment(h, namespace, name)
			Expect(err).ToNot(HaveOccurred(), "failed fetching deployment")

			readyReplicas := deployment.Status.ReadyReplicas
			desiredReplicas := deployment.Status.Replicas

			// The desired replicas should match the default installed replica count
			Expect(desiredReplicas).To(BeNumerically("==", defaultDesiredReplicas), "The deployment desired replicas should not drift from the default 1.")

			// Desired replica count should match ready replica count
			Expect(readyReplicas).To(BeNumerically("==", desiredReplicas), "All desired replicas should be ready.")
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func checkClusterRoles(h *helper.H, clusterRoles []string) {
	// Check that the clusterRoles exist
	ginkgo.Context("clusterRoles", func() {
		ginkgo.It("should exist", func() {
			for _, clusterRoleName := range clusterRoles {
				_, err := h.Kube().RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred(), "failed to get clusterRole %v\n", clusterRoleName)
			}
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func checkClusterRoleBindings(h *helper.H, clusterRoleBindings []string) {
	// Check that the clusterRoleBindings exist
	ginkgo.Context("clusterRoleBindings", func() {
		ginkgo.It("should exist", func() {
			for _, clusterRoleBindingName := range clusterRoleBindings {
				err := pollClusterRoleBinding(h, clusterRoleBindingName)
				Expect(err).ToNot(HaveOccurred(), "failed to get clusterRoleBinding %v\n", clusterRoleBindingName)
			}
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func checkRole(h *helper.H, namespace string, roles []string) {
	// Check that deployed roles exist
	ginkgo.Context("roles", func() {
		ginkgo.It("should exist", func() {
			for _, roleName := range roles {
				_, err := h.Kube().RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "failed to get role %v\n", roleName)
			}
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})

}

func checkRoleBindings(h *helper.H, namespace string, roleBindings []string) {
	// Check that deployed rolebindings exist
	ginkgo.Context("roleBindings", func() {
		ginkgo.It("should exist", func() {
			for _, roleBindingName := range roleBindings {
				err := pollRoleBinding(h, namespace, roleBindingName)
				Expect(err).NotTo(HaveOccurred(), "failed to get roleBinding %v\n", roleBindingName)
			}
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

//nolint
func checkSecrets(h *helper.H, namespace string, secrets []string) {
	// Check that deployed secrets exist
	ginkgo.Context("secrets", func() {
		ginkgo.It("should exist", func() {
			for _, secretName := range secrets {
				_, err := h.Kube().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "failed to get secret %v\n", secretName)
			}
		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func getInstallPlan(h *helper.H, sub *operatorv1.Subscription) (*operatorv1.InstallPlan, error) {
	subNamespace := sub.Namespace
	subName := sub.Name
	err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		s, err := h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Get(context.TODO(), subName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if s.Status.InstallPlanRef != nil {
			sub = s
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return h.Operator().OperatorsV1alpha1().InstallPlans(subNamespace).Get(context.TODO(), sub.Status.InstallPlanRef.Name, metav1.GetOptions{})
}

func approveInstallPlan(h *helper.H, ip *operatorv1.InstallPlan) error {
	ip.Spec.Approved = true
	_, err := h.Operator().OperatorsV1alpha1().InstallPlans(ip.Namespace).Update(context.TODO(), ip, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func ensureCSVIsInstalled(h *helper.H, csvName string, namespace string) error {
	err := wait.PollImmediate(5*time.Second, 15*time.Minute, func() (bool, error) {
		csv, err := h.Operator().OperatorsV1alpha1().ClusterServiceVersions(namespace).Get(context.TODO(), csvName, metav1.GetOptions{})
		if err != nil && !kerror.IsNotFound(err) {
			return false, err
		}

		if csv.Status.Phase == operatorv1.CSVPhaseSucceeded {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

func checkUpgrade(h *helper.H, subNamespace string, subName string, previousCSV string) {
	ginkgo.Context("Operator Upgrade", func() {
		ginkgo.It("should upgrade from the replaced version", func() {

			sub, err := h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Get(context.TODO(), subName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed trying to get Subscription %s in %s namespace", subName, subNamespace))
			startingCSV := sub.Status.CurrentCSV

			// Delete current Operator installation
			err = h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Delete(context.TODO(), subName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed trying to delete Subscription %s", subName))
			err = h.Operator().OperatorsV1alpha1().ClusterServiceVersions(subNamespace).Delete(context.TODO(), startingCSV, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed trying to delete ClusterServiceVersion %s", startingCSV))

			Eventually(func() bool {
				_, err := h.Operator().OperatorsV1alpha1().InstallPlans(subNamespace).Get(context.TODO(), sub.Status.Install.Name, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "installplan never garbage collected")

			// Create subscription to the previous version
			sub, err = h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Create(context.TODO(), &operatorv1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subName,
					Namespace: subNamespace,
				},
				Spec: &operatorv1.SubscriptionSpec{
					Package:                sub.Spec.Package,
					Channel:                sub.Spec.Channel,
					CatalogSourceNamespace: sub.Spec.CatalogSourceNamespace,
					CatalogSource:          sub.Spec.CatalogSource,
					InstallPlanApproval:    operatorv1.ApprovalManual,
					StartingCSV:            previousCSV,
				},
			}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed trying to create Subscription %s", subName))

			// Approve and manually verify the first installation to previousCSV
			ip, err := getInstallPlan(h, sub)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed getting InstallPlan for CSV %s", previousCSV))
			err = approveInstallPlan(h, ip)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed approving InstallPlan for CSV %s", previousCSV))
			err = ensureCSVIsInstalled(h, previousCSV, subNamespace)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("CSV %s did not install successfully", previousCSV))

			// Update the Subscription to apply Automatic updates from now on in order to reach currentCSV
			sub, err = h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Get(context.TODO(), subName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed retrieving updated subscription for %s", subName))
			sub.Spec.InstallPlanApproval = operatorv1.ApprovalAutomatic
			sub, err = h.Operator().OperatorsV1alpha1().Subscriptions(subNamespace).Update(context.TODO(), sub, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed updating subscription to Automatic for CSV %s", previousCSV))

			// The previous CSV is now installed and a new InstallPlan is also created ready for approval to upgrade to startingCSV
			// Approve and verify install to startingCSV
			ip, err = getInstallPlan(h, sub)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed getting InstallPlan for CSV %s", startingCSV))
			err = approveInstallPlan(h, ip)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed approving InstallPlan for CSV %s", startingCSV))
			err = ensureCSVIsInstalled(h, startingCSV, subNamespace)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("CSV %s did not install successfully", startingCSV))

		}, float64(viper.GetFloat64(config.Tests.PollingTimeout)))
	})
}

func pollClusterRoleBinding(h *helper.H, clusterRoleBindingName string) error {
	// pollRoleBinding will check for the existence of a clusterRole
	// in the specified project, and wait for it to exist, until a timeout

	var err error
	// interval is the duration in seconds between polls
	// values here for humans

	interval := 5

	// convert time.Duration type
	timeoutDuration := time.Duration(viper.GetFloat64(config.Tests.PollingTimeout)) * time.Minute
	intervalDuration := time.Duration(interval) * time.Second

	start := time.Now()

Loop:
	for {
		_, err = h.Kube().RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleBindingName, metav1.GetOptions{})
		elapsed := time.Since(start)

		switch {
		case err == nil:
			// Success
			break Loop
		case strings.Contains(err.Error(), "forbidden"):
			return err
		default:
			if elapsed < timeoutDuration {
				log.Printf("Waiting %v for %s clusterRoleBinding to exist", (timeoutDuration - elapsed), clusterRoleBindingName)
				time.Sleep(intervalDuration)
			} else {
				err = fmt.Errorf("Failed to get clusterRolebinding %s before timeout", clusterRoleBindingName)
				break Loop
			}
		}
	}

	return err
}

func pollRoleBinding(h *helper.H, projectName string, roleBindingName string) error {
	// pollRoleBinding will check for the existence of a roleBinding
	// in the specified project, and wait for it to exist, until a timeout

	var err error
	// interval is the duration in seconds between polls
	// values here for humans

	interval := 5

	// convert time.Duration type
	timeoutDuration := time.Duration(viper.GetFloat64(config.Tests.PollingTimeout)) * time.Minute
	intervalDuration := time.Duration(interval) * time.Second

	start := time.Now()

Loop:
	for {
		_, err = h.Kube().RbacV1().RoleBindings(projectName).Get(context.TODO(), roleBindingName, metav1.GetOptions{})
		elapsed := time.Since(start)

		switch {
		case err == nil:
			// Success
			break Loop
		case strings.Contains(err.Error(), "forbidden"):
			return err
		default:
			if elapsed < timeoutDuration {
				log.Printf("Waiting %v for %s roleBinding to exist", (timeoutDuration - elapsed), roleBindingName)
				time.Sleep(intervalDuration)
			} else {
				err = fmt.Errorf("Failed to get rolebinding %s before timeout", roleBindingName)
				break Loop
			}
		}
	}

	return err
}

func pollLockFile(h *helper.H, namespace, operatorLockFile string) error {
	// GetConfigMap polls for a configMap with a timeout
	// to handle the case when a new cluster is up but the OLM has not yet
	// finished deploying the operator

	var err error

	// interval is the duration in seconds between polls
	// values here for humans
	interval := 30

	// convert time.Duration type
	timeoutDuration := time.Duration(viper.GetFloat64(config.Tests.PollingTimeout)) * time.Minute
	intervalDuration := time.Duration(interval) * time.Second

	start := time.Now()

Loop:
	for {
		_, err = h.Kube().CoreV1().ConfigMaps(namespace).Get(context.TODO(), operatorLockFile, metav1.GetOptions{})
		elapsed := time.Since(start)

		switch {
		case err == nil:
			// Success
			break Loop
		case strings.Contains(err.Error(), "forbidden"):
			return err
		default:
			if elapsed < timeoutDuration {
				log.Printf("Waiting %v for %s configMap to exist", (timeoutDuration - elapsed), operatorLockFile)
				time.Sleep(intervalDuration)
			} else {
				err = fmt.Errorf("Failed to get configMap %s before timeout", operatorLockFile)
				break Loop
			}
		}
	}

	return err
}

func pollDeployment(h *helper.H, namespace, deploymentName string) (*appsv1.Deployment, error) {
	// pollDeployment polls for a deployment with a timeout
	// to handle the case when a new cluster is up but the OLM has not yet
	// finished deploying the operator

	var err error
	var deployment *appsv1.Deployment

	// interval is the duration in seconds between polls
	// values here for humans
	interval := 5

	// convert time.Duration type
	timeoutDuration := time.Duration(viper.GetFloat64(config.Tests.PollingTimeout)) * time.Minute
	intervalDuration := time.Duration(interval) * time.Second

	start := time.Now()

Loop:
	for {
		deployment, err = h.Kube().AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		elapsed := time.Since(start)

		switch {
		case err == nil:
			// Success
			break Loop
		case strings.Contains(err.Error(), "forbidden"):
			return nil, err
		default:
			if elapsed < timeoutDuration {
				log.Printf("Waiting %v for %s deployment to exist", (timeoutDuration - elapsed), deploymentName)
				time.Sleep(intervalDuration)
			} else {
				deployment = nil
				err = fmt.Errorf("Failed to get %s Deployment before timeout", deploymentName)
				break Loop
			}
		}
	}

	return deployment, err
}

func pollCsvList(h *helper.H, namespace, csvDisplayName string) (*operatorv1.ClusterServiceVersionList, error) {
	// pollCsvList polls for clusterServiceVersions with a timeout
	// to handle the case when a new cluster is up but the OLM has not yet
	// finished deploying the operator

	var err error
	var csvList *operatorv1.ClusterServiceVersionList

	// interval is the duration in seconds between polls
	// values here for humans
	interval := 5

	// convert time.Duration type
	timeoutDuration := time.Duration(viper.GetFloat64(config.Tests.PollingTimeout)) * time.Minute
	intervalDuration := time.Duration(interval) * time.Second

	start := time.Now()

Loop:
	for {
		csvList, err = h.Operator().OperatorsV1alpha1().ClusterServiceVersions(namespace).List(context.TODO(), metav1.ListOptions{})
		for _, csv := range csvList.Items {
			switch {
			case csvDisplayName == csv.Spec.DisplayName:
				// Success
				err = nil
			default:
				err = fmt.Errorf("No matching clusterServiceVersion in CSV List")
			}
		}
		elapsed := time.Since(start)

		switch {
		case err == nil:
			// Success
			break Loop
		case strings.Contains(err.Error(), "forbidden"):
			return nil, err
		default:
			if elapsed < timeoutDuration {
				log.Printf("Waiting %v for %s clusterServiceVersion to exist", (timeoutDuration - elapsed), csvDisplayName)
				time.Sleep(intervalDuration)
			} else {
				csvList = nil
				err = fmt.Errorf("Failed to get %s clusterServiceVersion before timeout", csvDisplayName)
				break Loop
			}
		}
	}

	return csvList, err
}
