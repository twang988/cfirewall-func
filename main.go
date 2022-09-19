package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func Run(rl *fn.ResourceList) (bool, error) {

	// find CfwConfig config
	var cfw_config CfwConfig
	if err := rl.FunctionConfig.As(&cfw_config); err != nil {
		return false, fmt.Errorf("could not parse function config: %s", err)
	}
	//fmt.Printf("-------------%+v-----------\n", cf)
	//return true, fmt.Errorf("-------------%+v-----------\n", cfw_config)

	//if obj.IsGVK("", "v1", "ConfigMap") && obj.GetAnnotation("config.kubernetes.io/local-config") == "true" {
	//	continue
	//}

	// process resource
	// Treat as local resource first
	if len(cfw_config.ConfigMaps) != 2 {
		// if configMaps in funcConfig number not not match, process as local config
		if err := mergeCrdAndCfwData(rl); err != nil {
			return false, err
		}
		return true, nil
	} else {
		//TODO: process remote packages
		return true, nil
	}

}
func makeHostDevConfig(ifname string) string {
	devcfg := make(map[string]string)
	devcfg["cniVersion"] = "0.3.0"
	devcfg["type"] = "host-device"
	devcfg["device"] = ifname
	jsonStr, _ := json.Marshal(devcfg)
	return string(jsonStr)
}

func mergeCrdAndCfwData(rl *fn.ResourceList) error {

	// map unprotectedNetPortVfw and protectedNetPortVfw
	// from crd yaml to deployment.yaml/template/metadata/annotations

	var cfwInfo CfwNetInfo
	//set ifname or datanet name for crd
	for _, obj := range rl.Items {
		if obj.IsGVK("k8s.cni.cncf.io", "v1", "NetworkAttachmentDefinition") &&
			obj.GetAnnotation("hostdev-protected-private-net-ifname") != "" {
			// Logic of processing protected hostdev crd
			ifname := obj.GetAnnotation("hostdev-protected-private-net-ifname")
			//obj.RemoveNestedFieldOrDie("metadata", "annotations", "hostdev-protected-private-net-ifname")

			crdName := "host-device-" + ifname
			obj.SetName(crdName)
			obj.SetNestedString(makeHostDevConfig(ifname), "spec", "config")
			cfwInfo.hostDevProtectedIfName = ifname

			_info := fmt.Sprintf("Change protected-private-net crd ifname to %s", ifname)
			rl.Results = append(rl.Results, fn.GeneralResult(_info, fn.Info))

		} else if obj.IsGVK("k8s.cni.cncf.io", "v1", "NetworkAttachmentDefinition") &&
			obj.GetAnnotation("hostdev-unprotected-private-net-ifname") != "" {
			// Logic of processing unprotected hostdev crd
			ifname := obj.GetAnnotation("hostdev-unprotected-private-net-ifname")
			//obj.RemoveNestedFieldOrDie("metadata", "annotations", "hostdev-unprotected-private-net-ifname")

			crdName := "host-device-" + ifname
			obj.SetName(crdName)
			obj.SetNestedString(makeHostDevConfig(ifname), "spec", "config")
			cfwInfo.hostDevUnprotectedIfName = ifname

			_info := fmt.Sprintf("Change unprotected-private-net crd ifname to %s", ifname)
			rl.Results = append(rl.Results, fn.GeneralResult(_info, fn.Info))

		} else if obj.IsGVK("k8s.cni.cncf.io", "v1", "NetworkAttachmentDefinition") &&
			obj.GetAnnotation("sriov-protected-net-providername") != "" {
			// Logic of processing protected sr-iov crd
		} else if obj.IsGVK("k8s.cni.cncf.io", "v1", "NetworkAttachmentDefinition") &&
			obj.GetAnnotation("sriov-unprotected-net-providername") != "" {
			// Logic of processing unprotected sr-iov crd
		}
	}

	//set ifname or datanet name for deployment
	for _, obj := range rl.Items {
		if obj.IsGVK("apps", "v1", "Deployment") &&
			obj.GetAnnotation("cfw-deployment") == "hostdev" {
			// Logic of processing cfirewall hostdev deployment
			if cfwInfo.hostDevProtectedIfName == "" || cfwInfo.hostDevUnprotectedIfName == "" {
				return fmt.Errorf("no valid CfwNetInfo:%+v", cfwInfo)
			}
			return nil

		} else if obj.IsGVK("apps", "v1", "Deployment") &&
			obj.GetAnnotation("cfw-deployment") == "sriov" {
			// Logic of processing cfirewall sriov deployment
			//TODO
			return nil
		}
	}
	//return fmt.Errorf("-----start--------%+v------end-----\n", rl)
	return fmt.Errorf("no valid cfw-deployment found")
}

type CfwNetInfo struct {
	hostDevProtectedIfName          string //ifname like eth21
	hostDevUnprotectedIfName        string //ifname like eth11
	sriovProtectedNetProviderName   string //datanet name like datanet_1
	sriovUnprotectedNetProviderName string //datanet name like datanet_1
}

type CfwConfig struct {
	yaml.ResourceIdentifier `json:",inline" yaml:",inline"`
	ConfigMaps              []configList `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
}

type configList struct {
	//type is coreFirewall or networkInfraCrd
	PkgType      PkgType      `yaml:"pkgtype,omitempty" json:"pkgtype,omitempty"`
	UpstreamLock UpstreamLock `yaml:"upstreamlock,omitempty" json:"upstreamlock,omitempty"`
}

// PkgType defines the type of cfirewall package usage.
type PkgType string

const (
	// corefirewall and networkinfcrd specifies a package usage.
	corefirewall  PkgType = "coreFirewall"
	networkinfcrd PkgType = "networkInfraCrd"
)

// OriginType defines the type of origin for a package.
type OriginType string

const (
	// GitOrigin specifies a package as having been cloned from a git repository.
	GitOrigin OriginType = "git"
)

type UpstreamLock struct {
	// Type is the type of origin.
	OriginType OriginType `yaml:"origintype,omitempty" json:"origintype,omitempty"`

	// Git is the resolved locator for a package on Git.
	Git GitLock `yaml:"gitlock,omitempty" json:"gitlock,omitempty"`
}

type GitLock struct {
	// Repo is the git repository that was fetched.
	// e.g. 'https://github.com/kubernetes/examples.git'
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`

	// Directory is the sub directory of the git repository that was fetched.
	// e.g. 'staging/cockroachdb'
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`

	// Ref can be a Git branch, tag, or a commit SHA-1 that was fetched.
	// e.g. 'master'
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`

	// Commit is the SHA-1 for the last fetch of the package.
	// This is set by kpt for bookkeeping purposes.
	Commit string `yaml:"commit,omitempty" json:"commit,omitempty"`

	//Proxy is git proxy
	Proxy string `yaml:"proxy,omitempty" json:"proxy,omitempty"`
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
