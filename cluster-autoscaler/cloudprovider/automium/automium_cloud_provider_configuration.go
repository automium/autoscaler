package automium

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// ReadProviderConfiguration populates the config
func ReadProviderConfiguration(path string) (CloudProviderConfig, error) {
	var theConf CloudProviderConfig

	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return theConf, err
	}

	err = yaml.Unmarshal(fileContent, &theConf)
	if err != nil {
		return theConf, err
	}

	err = validateConfig(theConf)
	if err != nil {
		return theConf, err
	}

	return theConf, nil
}

func validateConfig(conf CloudProviderConfig) error {
	if conf.ClusterName == "" {
		return errors.New("invalid cluster name in configuration")
	}

	if _, err := os.Stat(conf.AutomiumKubeConfigPath); os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("cannot stat Automium agent kubeconfig: %s", err.Error()))
	}

	return nil
}
