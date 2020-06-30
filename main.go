package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func findFiles(base string, glob glob.Glob) ([]string, error) {
	var paths []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
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

	return client.Create(ctx, u)
}

func main() {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		panic(err.Error())
	}

	client, err := client.New(config, client.Options{})
	if err != nil {
		klog.Fatal(err)
	}

	base := "."
	glob := glob.MustCompile("tekton/**.yaml")
	files, err := findFiles(base, glob)
	fmt.Println(files)

	ctx := context.Background()
	for _, f := range files {
		if err := create(ctx, client, f); err != nil {
			klog.Error(err)
		}
	}
}
