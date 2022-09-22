package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	// process resource
	// Treat as local resource if configmap number not match
	if len(cfw_config.ConfigMaps) == 0 {
		// if configMaps in funcConfig number not not match, process as local config
		if err := generate(rl, cfw_config); err != nil {
			return false, err
		}
		rl.Results = append(rl.Results, fn.GeneralResult("Using local file", fn.Info))
		return true, nil
	} else {
		rl.Results = append(rl.Results, fn.GeneralResult("Using remote git repos", fn.Info))
		if err := makeResourceListFromGit(cfw_config, rl); err != nil {
			return false, err
		}
		if err := generate(rl, cfw_config); err != nil {
			return false, err
		}
		return true, nil
	}

}
func cleanTmpGitLocalPath() {
	rmArgs := fmt.Sprintf("-rf %s", tmpGitLocalPath)
	args := strings.Fields(rmArgs)
	cmd := exec.Command("rm", args...)
	cmd.Run()
}
func switchGitTag(tag string) error {
	if tag != "" {
		argstr := fmt.Sprintf("checkout %s", tag)
		args := strings.Fields(argstr)
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpGitLocalPath
		return cmd.Run()
	}
	return nil
}
func setGitProxy(proxy string) error {
	if proxy != "" {
		argstr := fmt.Sprintf("config --global https.proxy %s", proxy)
		args := strings.Fields(argstr)
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpGitLocalPath
		err := cmd.Run()
		if err != nil {
			return err
		}
		argstr = fmt.Sprintf("config --global http.proxy %s", proxy)
		args = strings.Fields(argstr)
		cmd = exec.Command("git", args...)
		err = cmd.Run()
		if err != nil {
			return err
		}
	} else {
		//clean up provious proxy
		args := strings.Fields("config --global --unset http.proxy")
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpGitLocalPath
		err := cmd.Run()
		if err != nil {
			return err
		}
		args = strings.Fields("config --global --unset https.proxy")
		cmd = exec.Command("git", args...)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}
func findFileRecur(path string, suffix string) []string {
	// Find *.yaml or *.yml recursively
	var flist []string
	stdout := new(bytes.Buffer)
	argstr := fmt.Sprintf("%s -name *.%s", path, suffix)
	args := strings.Fields(argstr)
	cmd := exec.Command("find", args...)
	cmd.Stdout = stdout
	cmd.Dir = "/"
	err := cmd.Run()
	if err != nil {
		return flist
	}
	return strings.Fields(stdout.String())

}
func convertFolderToKubeObjs(path string) (obj fn.KubeObjects, err error) {
	flist := findFileRecur(path, "yaml")
	flist = append(flist, findFileRecur(path, "yml")...)
	for _, file := range flist {
		cont, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		o, err := fn.ParseKubeObject(cont)
		if err != nil {
			return nil, err
		}
		obj = append(obj, o)
	}
	return obj, nil
}
func makeResourceListFromGit(cfwcfg CfwConfig, rl *fn.ResourceList) error {
	for _, cm := range cfwcfg.ConfigMaps {
		cleanTmpGitLocalPath()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		gitArgs := fmt.Sprintf("clone %s %s", cm.UpstreamLock.Git.Repo, tmpGitLocalPath)
		args := strings.Fields(gitArgs)
		cmd := exec.Command("git", args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("unable to run: '%s %s' with env=%s; %q: %q",
				"git", args[0], "env", err.Error(), stderr.String())
		}
		err = switchGitTag(cm.UpstreamLock.Git.Ref)
		if err != nil {
			return fmt.Errorf("unable to switch tag to %s",
				cm.UpstreamLock.Git.Ref)
		}
		err = setGitProxy(cm.UpstreamLock.Git.Proxy)
		if err != nil {
			return fmt.Errorf("unable to setGitProxy %s",
				cm.UpstreamLock.Git.Proxy)
		}
		rl.Items, err = convertFolderToKubeObjs(tmpGitLocalPath)
		//fmt.Printf("------%+v------\n", rl.Items)
		if err != nil {
			return err
		}
	}

	return nil
}

func makeHostDevConfig(ifname string) string {
	devcfg := make(map[string]string)
	devcfg["cniVersion"] = "0.3.0"
	devcfg["type"] = "host-device"
	devcfg["device"] = ifname
	jsonStr, _ := json.Marshal(devcfg)
	return string(jsonStr)
}
func makeDeploymentNetworks(cfwcfg CfwConfig) string {
	var netList []map[string]string

	for _, netif := range cfwcfg.DeploymentSelector.NadIfnames {
		if netif.Type == hostdevType {
			netcfg := make(map[string]string)
			netcfg["name"] = hostdevPrefix + netif.Vdev
			netcfg["interface"] = netif.Vdev
			netList = append(netList, netcfg)
		} else if netif.Type == sriovType {
			//TODO
		}
	}
	jsonStr, _ := json.Marshal(netList)
	return string(jsonStr)
}
func generateNetCrd(cfwcfg CfwConfig, objs []*fn.KubeObject) ([]*fn.KubeObject, error) {
	for _, netif := range cfwcfg.DeploymentSelector.NadIfnames {

		if netif.Type == hostdevType {
			o := fn.NewEmptyKubeObject()
			o.SetAPIVersion("k8s.cni.cncf.io/v1")
			o.SetKind("NetworkAttachmentDefinition")
			o.SetName(hostdevPrefix + netif.Vdev)
			o.SetNamespace("example")
			o.SetAnnotation("networkname", netif.Networkname)
			o.SetNestedString(makeHostDevConfig(netif.Phydev), "spec", "config")
			//return nil, fmt.Errorf("---%+v--\n", o)

			objs = append(objs, o)
		} else if netif.Type == sriovType {
			//TODO
		}
	}
	return objs, nil
}

func generate(rl *fn.ResourceList, cfwcfg CfwConfig) error {

	//var cfwInfo CfwNetInfo

	// Generate Net CRDs
	var err error
	rl.Items, err = generateNetCrd(cfwcfg, rl.Items)
	if err != nil {
		return err
	}

	//return fmt.Errorf("\n###\n%+v\n###\n", rl.Items)

	//set ifname or datanet name for deployment

	// Get deployment labels to match
	depLabels := make(map[string]string)
	for _, label := range cfwcfg.DeploymentSelector.MatchLabels {
		depLabels[label.Key] = label.Val
	}

	for _, obj := range rl.Items {
		if obj.IsGVK("apps", "v1", "Deployment") &&
			obj.HasLabels(depLabels) {
			obj.SetNestedString(makeDeploymentNetworks(cfwcfg), "spec", "template",
				"metadata", "annotations", "k8s.v1.cni.cncf.io/networks")
		}
	}
	return nil
}

const (
	sriovPrefix     string = "sriov-device-"
	hostdevPrefix   string = "host-device-"
	tmpGitLocalPath string = "/tmp/git/"
	sriovType       string = "sriov"
	hostdevType     string = "hostdev"
)

type deploymentSelector struct {
	MatchLabels []MatchLabel `json:"matchLabels,omitempty" yaml:"matchLabels,omitempty"`
	NadIfnames  []NadIfname  `json:"NadIfnames,omitempty" yaml:"NadIfnames,omitempty"`
}
type MatchLabel struct {
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
	Val string `json:"val,omitempty" yaml:"val,omitempty"`
}

type NadIfname struct {
	Networkname string `json:"networkname,omitempty" yaml:"networkname,omitempty"`
	Phydev      string `json:"phydev,omitempty" yaml:"phydev,omitempty"`
	Vdev        string `json:"vdev,omitempty" yaml:"vdev,omitempty"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
}

type CfwConfig struct {
	yaml.ResourceIdentifier `json:",inline" yaml:",inline"`
	ConfigMaps              []configList       `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	DeploymentSelector      deploymentSelector `json:"deploymentSelector,omitempty" yaml:"deploymentSelector,omitempty"`
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
