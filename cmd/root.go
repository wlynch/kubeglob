package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	baseDir     string
	userGlob    string
	kubecfgPath string
	dryrun      bool

	rootCmd = &cobra.Command{
		Use:   "kubeglob",
		Short: "Create Kubernetes resource based on a file glob.",
		RunE:  run,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&baseDir, "base", "b", ".", "base directory to walk")
	rootCmd.PersistentFlags().StringVarP(&userGlob, "glob", "g", "*", "glob to match files on")
	rootCmd.PersistentFlags().StringVarP(&kubecfgPath, "kubeconfig", "c", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "kubeconfig path")
	rootCmd.PersistentFlags().BoolVarP(&dryrun, "dry-run", "n", false, "do not create resources, only print paths")
}

func findFiles(base string, glob glob.Glob) ([]string, error) {
	var paths []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && glob.Match(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func create(ctx context.Context, client client.Writer, path string) error {
	klog.Info(path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	u := new(unstructured.Unstructured)
	if err := yaml.Unmarshal(b, u); err != nil {
		return err
	}

	if u.GetNamespace() == "" {
		u.SetNamespace("default")
	}

	if dryrun {
		return nil
	}
	return client.Create(ctx, u)
}

func run(cmd *cobra.Command, args []string) error {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubecfgPath)
	if err != nil {
		klog.Error("error reading kubecfg:", err)
		return err
	}

	client, err := client.New(config, client.Options{})
	if err != nil {
		klog.Error("error creating kubeclient:", err)
		return err
	}

	glob, err := glob.Compile(userGlob)
	if err != nil {
		klog.Error("error parsing glob:", err)
		return err
	}

	files, err := findFiles(baseDir, glob)
	ctx := context.Background()
	for _, f := range files {
		if err := create(ctx, client, f); err != nil {
			klog.Error(err)
			return nil
		}
	}
	return nil
}

// Execute runs the command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
