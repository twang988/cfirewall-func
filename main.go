package main

import (
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	infrav1alpha1 "github.com/nephio-project/nephio-controller-poc/apis/infra/v1alpha1"
)

func Run(rl *fn.ResourceList) (bool, error) {

	// find CfwConfig config
	var cf *fn.KubeObject
	for _, obj := range rl.Items {
		if obj.IsGVK(infrav1alpha1.GroupVersion.Group, infrav1alpha1.GroupVersion.Version, "CfwConfig") {
			//fmt.Printf("-------------%+v-----------\n", obj)
			//fmt.Printf("-------------%s-----------\n", reflect.obj.Method(0))
			cf = obj
			continue
		}
	}
	if cf == nil {
		return false, fmt.Errorf("could not find %s/%s CfwConfig",
			infrav1alpha1.GroupVersion.Group, infrav1alpha1.GroupVersion.Version)
	}
	if err := applyCfwConfig(rl, cf); err != nil {
		return false, err
	}
	return true, nil
}

func applyCfwConfig(rl *fn.ResourceList, cf *fn.KubeObject) error {
	networkType := cf.NestedStringOrDie("networkInfra", "type")
	if networkType == "" {
		return fmt.Errorf("could not find %s/%s in CfwConfig",
			"networkInfra", "type")
	}

	fmt.Printf("-------------%+v-----------\n", networkType)
	cf.SetAnnotation("key", "avalue")
	return nil
}
func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
