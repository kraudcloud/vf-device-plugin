package main

import (
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

type vfConfig struct {
}

func readConfigFile(fileName string) vfConfig {
	var config vfConfig

	yamlFile, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Fatal(err)
	}

	return config
}
