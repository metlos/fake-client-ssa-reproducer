package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/csaupgrade"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, "Pass the name of the namespace as the argument\n")
		os.Exit(1)
	}

	cl := getClient()
	fcl := getFakeClient()

	ns := os.Args[1]

	cm := runPayload(cl, ns)
	fcm := runPayload(fcl, ns)

	fmt.Println("The difference between managed fields of object in the real cluster vs fake client:")

	fmt.Println(cmp.Diff(cm.ManagedFields, fcm.ManagedFields, cmp.FilterPath(func(p cmp.Path) bool {
		return p.String() == "Time"
	}, cmp.Ignore())))
}

func runPayload(cl client.Client, ns string) *corev1.ConfigMap {
	// cleanup
	cm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), client.ObjectKey{Name: "cm", Namespace: ns}, cm); err != nil {
		if !errors.IsNotFound(err) {
			exitIfError(err)
		}
	} else {
		exitIfError(cl.Delete(context.TODO(), cm))
	}

	// actual test
	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "cm",
			Namespace:     ns,
			ManagedFields: nil,
		},
		Data: map[string]string{
			"firstKey": "firstValue",
		},
	}

	exitIfError(cl.Create(context.TODO(), cm))

	key := client.ObjectKeyFromObject(cm)
	cm = &corev1.ConfigMap{}
	exitIfError(cl.Get(context.TODO(), key, cm))

	oldFieldOwner := strings.Split(rest.DefaultKubernetesUserAgent(), "/")[0]
	cmCopy := cm.DeepCopy()
	exitIfError(csaupgrade.UpgradeManagedFields(cmCopy, sets.New(oldFieldOwner), "alice"))

	// neither works correctly:
	//
	// exitIfError(cl.Update(context.TODO(), cm))
	exitIfError(cl.Patch(context.TODO(), cmCopy, client.MergeFrom(cm)))

	key = client.ObjectKeyFromObject(cm)
	cm = &corev1.ConfigMap{}
	exitIfError(cl.Get(context.TODO(), key, cm))

	return cm
}

func getFakeClient() client.Client {
	scheme := runtime.NewScheme()
	exitIfError(corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithReturnManagedFields().WithScheme(scheme).Build()
}

func getClient() client.Client {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	exitIfError(err)

	restCfg, err := clientcmd.NewDefaultClientConfig(*cfg, nil).ClientConfig()
	exitIfError(err)

	scheme := runtime.NewScheme()
	exitIfError(corev1.AddToScheme(scheme))

	discoverCl, err := discovery.NewDiscoveryClientForConfig(restCfg)
	exitIfError(err)

	grs, err := restmapper.GetAPIGroupResources(discoverCl)
	exitIfError(err)

	cl, err := client.New(restCfg, client.Options{
		Scheme: scheme,
		Mapper: restmapper.NewDiscoveryRESTMapper(grs),
	})
	exitIfError(err)

	return cl
}

func exitIfError(err error) {
	if err != nil {
		panic(fmt.Errorf("error (%T): %w", err, err))
	}
}
