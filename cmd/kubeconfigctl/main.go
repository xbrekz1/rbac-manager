package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	version = "1.0.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kubeconfigctl",
	Short: "Simple CLI tool for generating kubeconfig files",
	Long: `kubeconfigctl generates kubeconfig files for ServiceAccounts created by RBAC Manager.

It creates a service account token and packages it into a ready-to-use kubeconfig file.

Examples:
  # Generate kubeconfig for an AccessGrant
  kubeconfigctl generate john-dev

  # Generate with custom token duration (default: 8760h = 1 year)
  kubeconfigctl generate john-dev --duration 720h

  # Generate with custom output directory
  kubeconfigctl generate john-dev --output ~/Desktop

  # List all AccessGrants
  kubeconfigctl list

For more information, visit: https://github.com/xbrekz1/rbac-manager`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(listCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate [AccessGrant name]",
	Short: "Generate kubeconfig for an AccessGrant",
	Long: `Generate a kubeconfig file for the specified AccessGrant.

This command:
1. Finds the ServiceAccount associated with the AccessGrant
2. Creates a service account token with specified duration
3. Generates a kubeconfig file with cluster information
4. Saves it to the output directory (default: ~/Downloads)

The generated kubeconfig contains a token that is valid for the specified
duration (default: 8760h = 1 year). The token will work until it expires
or until you delete the AccessGrant/ServiceAccount.

Examples:
  # Generate with default settings (1 year token)
  kubeconfigctl generate john-dev

  # Generate with 30-day token
  kubeconfigctl generate john-dev --duration 720h

  # Generate with custom namespace and output location
  kubeconfigctl generate john-dev -n production -o /tmp

  # Generate with custom default namespace in kubeconfig
  kubeconfigctl generate john-dev --default-namespace development`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerate,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all AccessGrants",
	Long: `List all AccessGrants in the specified namespace.

This helps you see which AccessGrants are available for kubeconfig generation.

Examples:
  # List AccessGrants in default namespace (rbac-manager)
  kubeconfigctl list

  # List in specific namespace
  kubeconfigctl list --namespace production

  # List in all namespaces
  kubeconfigctl list --all-namespaces`,
	RunE: runList,
}

// Command flags
var (
	// Global flags
	namespace     string
	kubeconfig    string
	allNamespaces bool

	// Generate flags
	duration         string
	outputDir        string
	defaultNamespace string
	clusterName      string
	serverURL        string
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "rbac-manager", "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")

	// Generate flags
	generateCmd.Flags().StringVarP(&duration, "duration", "d", "8760h", "Token duration (e.g., 1h, 24h, 720h, 8760h)")
	generateCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (default: ~/Downloads)")
	generateCmd.Flags().StringVar(&defaultNamespace, "default-namespace", "default", "Default namespace in kubeconfig context")
	generateCmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster name in kubeconfig (default: from current context)")
	generateCmd.Flags().StringVar(&serverURL, "server", "", "Kubernetes API server URL (default: from current context)")

	// List flags
	listCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List AccessGrants across all namespaces")
}

func runGenerate(_ *cobra.Command, args []string) error {
	accessGrantName := args[0]

	fmt.Printf("🚀 Generating kubeconfig for AccessGrant: %s\n\n", accessGrantName)

	// Get Kubernetes client
	clientset, config, err := getK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Get AccessGrant information
	fmt.Printf("📋 Step 1/5: Fetching AccessGrant information...\n")
	saName, saNamespace, role, err := getAccessGrantInfo(accessGrantName, namespace)
	if err != nil {
		return err
	}

	fmt.Printf("   ✓ AccessGrant: %s\n", accessGrantName)
	fmt.Printf("   ✓ Role: %s\n", role)
	fmt.Printf("   ✓ ServiceAccount: %s (namespace: %s)\n\n", saName, saNamespace)

	// Verify ServiceAccount exists
	fmt.Printf("🔍 Step 2/5: Verifying ServiceAccount exists...\n")
	if err := verifyServiceAccount(clientset, saName, saNamespace); err != nil {
		return err
	}
	fmt.Printf("   ✓ ServiceAccount verified\n\n")

	// Create token
	fmt.Printf("🔑 Step 3/5: Creating service account token (duration: %s)...\n", duration)
	token, err := createToken(saName, saNamespace, duration)
	if err != nil {
		return err
	}
	fmt.Printf("   ✓ Token created successfully\n\n")

	// Get cluster information
	fmt.Printf("🌐 Step 4/5: Getting cluster information...\n")
	clusterInfo, err := getClusterInfo(config)
	if err != nil {
		return err
	}

	// Override with custom values if provided
	if clusterName != "" {
		clusterInfo.ClusterName = clusterName
	}
	if serverURL != "" {
		clusterInfo.ServerURL = serverURL
	}

	fmt.Printf("   ✓ Cluster: %s\n", clusterInfo.ClusterName)
	fmt.Printf("   ✓ Server: %s\n\n", clusterInfo.ServerURL)

	// Generate kubeconfig
	fmt.Printf("📝 Step 5/5: Generating kubeconfig file...\n")
	kubeconfigData, err := buildKubeconfig(buildKubeconfigParams{
		ClusterName:      clusterInfo.ClusterName,
		ServerURL:        clusterInfo.ServerURL,
		CAData:           clusterInfo.CAData,
		Token:            token,
		UserName:         saName,
		DefaultNamespace: defaultNamespace,
		ContextName:      fmt.Sprintf("%s-context", saName),
	})
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %v", err)
	}

	// Determine output directory
	outDir := outputDir
	if outDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}
		outDir = filepath.Join(homeDir, "Downloads")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Save kubeconfig
	filename := fmt.Sprintf("kubeconfig-%s.yaml", accessGrantName)
	outputPath := filepath.Join(outDir, filename)

	if err := os.WriteFile(outputPath, kubeconfigData, 0o600); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %v", err)
	}

	fmt.Printf("   ✓ Kubeconfig saved: %s\n\n", outputPath)

	// Generate instructions file
	instructionsPath := filepath.Join(outDir, fmt.Sprintf("kubeconfig-%s-instructions.txt", accessGrantName))
	instructions := generateInstructions(accessGrantName, saName, role, duration, clusterInfo.ClusterName, outputPath)
	if err := os.WriteFile(instructionsPath, []byte(instructions), 0o600); err != nil {
		fmt.Printf("   ⚠ Warning: Could not create instructions file: %v\n", err)
	} else {
		fmt.Printf("   ✓ Instructions saved: %s\n\n", instructionsPath)
	}

	// Success message
	fmt.Printf("✅ Kubeconfig generated successfully!\n\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📁 Location: %s\n", outputPath)
	fmt.Printf("⏱️  Token Duration: %s\n", duration)
	fmt.Printf("🔐 Role: %s\n", role)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	fmt.Printf("To use this kubeconfig:\n\n")
	fmt.Printf("  export KUBECONFIG=%s\n", outputPath)
	fmt.Printf("  kubectl auth whoami\n")
	fmt.Printf("  kubectl get namespaces\n\n")

	fmt.Printf("To test permissions:\n\n")
	fmt.Printf("  kubectl auth can-i get pods\n")
	fmt.Printf("  kubectl auth can-i --list\n\n")

	return nil
}

func runList(_ *cobra.Command, _ []string) error {
	// Get Kubernetes client
	clientset, _, err := getK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	ctx := context.Background()
	var accessGrants []AccessGrantInfo

	if allNamespaces {
		// List in all namespaces
		fmt.Println("🔍 Listing AccessGrants in all namespaces...")

		// Get all namespaces
		nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list namespaces: %v", err)
		}

		for i := range nsList.Items {
			ags, err := listAccessGrantsInNamespace(nsList.Items[i].Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list AccessGrants in namespace %q: %v\n", nsList.Items[i].Name, err)
				continue
			}
			accessGrants = append(accessGrants, ags...)
		}
	} else {
		// List in specific namespace
		fmt.Printf("🔍 Listing AccessGrants in namespace: %s\n\n", namespace)
		accessGrants, err = listAccessGrantsInNamespace(namespace)
		if err != nil {
			return err
		}
	}

	if len(accessGrants) == 0 {
		fmt.Println("No AccessGrants found.")
		return nil
	}

	// Print table
	if allNamespaces {
		fmt.Printf("%-30s %-20s %-30s %-15s %-10s\n", "NAMESPACE", "NAME", "SERVICEACCOUNT", "ROLE", "PHASE")
		fmt.Println(strings.Repeat("─", 110))
		for _, ag := range accessGrants {
			fmt.Printf("%-30s %-20s %-30s %-15s %-10s\n",
				ag.Namespace,
				ag.Name,
				ag.ServiceAccount,
				ag.Role,
				ag.Phase,
			)
		}
	} else {
		fmt.Printf("%-30s %-30s %-15s %-10s\n", "NAME", "SERVICEACCOUNT", "ROLE", "PHASE")
		fmt.Println(strings.Repeat("─", 90))
		for _, ag := range accessGrants {
			fmt.Printf("%-30s %-30s %-15s %-10s\n",
				ag.Name,
				ag.ServiceAccount,
				ag.Role,
				ag.Phase,
			)
		}
	}

	fmt.Printf("\nTotal: %d AccessGrant(s)\n", len(accessGrants))
	fmt.Printf("\nTo generate kubeconfig for an AccessGrant:\n")
	fmt.Printf("  kubeconfigctl generate <name> -n <namespace>\n\n")

	return nil
}

// Helper types and functions

type ClusterInfo struct {
	ClusterName string
	ServerURL   string
	CAData      string
}

type AccessGrantInfo struct {
	Name           string
	Namespace      string
	ServiceAccount string
	Role           string
	Phase          string
}

type buildKubeconfigParams struct {
	ClusterName      string
	ServerURL        string
	CAData           string
	Token            string
	UserName         string
	DefaultNamespace string
	ContextName      string
}

func getK8sClient() (*kubernetes.Clientset, *clientcmdapi.Config, error) {
	kubeconfigPath := kubeconfig
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, nil, err
			}
			kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
		}
	}

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, nil, err
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}

func getAccessGrantInfo(name, namespace string) (saName, saNamespace, role string, err error) {
	// Use kubectl to get AccessGrant info
	cmd := exec.Command("kubectl", "get", "accessgrant", name, "-n", namespace,
		"-o", "jsonpath={.status.serviceAccount},{.metadata.namespace},{.spec.role},{.status.phase}")

	output, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("AccessGrant %q not found in namespace %q: %v", name, namespace, err)
	}

	parts := strings.Split(string(output), ",")
	if len(parts) < 4 {
		return "", "", "", fmt.Errorf("invalid AccessGrant data")
	}

	saName = strings.TrimSpace(parts[0])
	saNamespace = strings.TrimSpace(parts[1])
	role = strings.TrimSpace(parts[2])
	phase := strings.TrimSpace(parts[3])

	// Fallback if status.serviceAccount is empty
	if saName == "" {
		cmd = exec.Command("kubectl", "get", "accessgrant", name, "-n", namespace,
			"-o", "jsonpath={.spec.serviceAccountName}")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			saName = strings.TrimSpace(string(output))
		}
	}

	// Final fallback
	if saName == "" {
		saName = "rbac-" + name
		fmt.Fprintf(os.Stderr, "Warning: ServiceAccount not yet set in status, using default name %q. Ensure AccessGrant is Active.\n", saName)
	}

	// Check if AccessGrant is Active
	if phase != "Active" {
		return "", "", "", fmt.Errorf("AccessGrant is not in Active phase (current: %s)", phase)
	}

	return saName, saNamespace, role, nil
}

func verifyServiceAccount(clientset *kubernetes.Clientset, saName, namespace string) error {
	ctx := context.Background()
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ServiceAccount %q not found in namespace %q: %v", saName, namespace, err)
	}
	return nil
}

func createToken(saName, namespace, duration string) (string, error) {
	// Use kubectl to create token
	cmd := exec.Command("kubectl", "create", "token", saName, "-n", namespace, "--duration", duration)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create token: %v", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("received empty token")
	}

	return token, nil
}

func getClusterInfo(config *clientcmdapi.Config) (*ClusterInfo, error) {
	// Get current context
	currentContext := config.CurrentContext
	if currentContext == "" {
		return nil, fmt.Errorf("no current context set in kubeconfig")
	}

	clusterCtx, ok := config.Contexts[currentContext]
	if !ok {
		return nil, fmt.Errorf("context %q not found", currentContext)
	}

	cluster, ok := config.Clusters[clusterCtx.Cluster]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterCtx.Cluster)
	}

	// Get CA data
	var caData string
	if len(cluster.CertificateAuthorityData) > 0 {
		caData = base64.StdEncoding.EncodeToString(cluster.CertificateAuthorityData)
	} else {
		return nil, fmt.Errorf("no certificate authority data found")
	}

	return &ClusterInfo{
		ClusterName: clusterCtx.Cluster,
		ServerURL:   cluster.Server,
		CAData:      caData,
	}, nil
}

func buildKubeconfig(params buildKubeconfigParams) ([]byte, error) {
	// Decode CA data
	caData, err := base64.StdEncoding.DecodeString(params.CAData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CA data: %v", err)
	}

	config := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			params.ClusterName: {
				Server:                   params.ServerURL,
				CertificateAuthorityData: caData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			params.ContextName: {
				Cluster:   params.ClusterName,
				AuthInfo:  params.UserName,
				Namespace: params.DefaultNamespace,
			},
		},
		CurrentContext: params.ContextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			params.UserName: {
				Token: params.Token,
			},
		},
	}

	return clientcmd.Write(config)
}

func listAccessGrantsInNamespace(namespace string) ([]AccessGrantInfo, error) {
	cmd := exec.Command("kubectl", "get", "accessgrants", "-n", namespace,
		"-o", "jsonpath={range .items[*]}{.metadata.name},{.status.serviceAccount},{.spec.role},{.status.phase},{.metadata.namespace}{'\\n'}{end}")

	output, _ := cmd.Output() // kubectl error means no AccessGrants found — treat as empty

	var accessGrants []AccessGrantInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) >= 4 {
			saName := strings.TrimSpace(parts[1])
			if saName == "" {
				saName = "rbac-" + strings.TrimSpace(parts[0])
			}

			accessGrants = append(accessGrants, AccessGrantInfo{
				Name:           strings.TrimSpace(parts[0]),
				ServiceAccount: saName,
				Role:           strings.TrimSpace(parts[2]),
				Phase:          strings.TrimSpace(parts[3]),
				Namespace:      namespace,
			})
		}
	}

	return accessGrants, nil
}

func generateInstructions(accessGrantName, saName, role, duration, clusterName, kubeconfigPath string) string {
	return fmt.Sprintf(`=======================================================
KUBECONFIG for AccessGrant: %s
=======================================================

Config file:    %s
Created:        %s
Token duration: %s
ServiceAccount: %s
Role:           %s
Cluster:        %s

USAGE:
------

1. Export KUBECONFIG:
   export KUBECONFIG=%s

2. Or use with flag:
   kubectl --kubeconfig=%s get pods

3. Verify connectivity:
   kubectl auth whoami
   kubectl get namespaces

4. Check permissions:
   kubectl auth can-i get pods
   kubectl auth can-i --list

SECURITY:
---------

WARNING: This file contains a cluster access token!
   - Do not share via unencrypted channels
   - Do not commit to git repositories
   - File permissions are set to 600 (owner read/write only)
   - Delete this file when no longer needed or if compromised

Role: %s
   - Access depends on the assigned role
   - See RBAC Manager documentation for role details

REVOKE ACCESS:
--------------

To revoke access, delete the AccessGrant:
   kubectl delete accessgrant %s -n rbac-manager

To renew the token (generate a new kubeconfig):
   kubeconfigctl generate %s

SUPPORT:
--------

If you encounter issues, contact your cluster administrator.
AccessGrant: %s
ServiceAccount: %s

=======================================================
`, accessGrantName, kubeconfigPath, time.Now().Format(time.RFC3339), duration, saName, role, clusterName,
		kubeconfigPath, kubeconfigPath, role, accessGrantName, accessGrantName, accessGrantName, saName)
}
