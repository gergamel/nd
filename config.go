package main

import (
	"fmt"
	"os"
	"log"
	"reflect"
	"strings"
)

// Configuration holds application configuration. Values will be pulled from
// environment variables, prefixed by keyPrefix. Default values can be added
// via tags.
type Configuration struct {
	Listen		string `config:"tcp://:8080"`
	Host		string `config:"localhost:8080"`
	DataPath	string `config:"/var/opt/ndel/"`
	AdminUser	string `config:""`
	AdminPass	string `config:""`
	Cert		string `config:""`
	Key		string `config:""`
	Proto		string `config:"http"`
}

func (c *Configuration) IsHTTPS() bool {
	return strings.Contains(Config.Proto, "https")
}

// Config is the global app configuration
var Config = &Configuration{}

const keyPrefix = "ND"

func init() {
	te := reflect.TypeOf(Config).Elem()
	ve := reflect.ValueOf(Config).Elem()

	for i := 0; i < te.NumField(); i++ {
		sf := te.Field(i)
		name := sf.Name
		field := ve.FieldByName(name)

		envVar := strings.ToUpper(fmt.Sprintf("%s_%s", keyPrefix, name))
		env := os.Getenv(envVar)
		tag := sf.Tag.Get("config")
		
		if env != "" {
			log.Printf("CONFIG:%s: %s set to %s from env", envVar, name, env)
		} else if tag != "" {
			log.Printf("CONFIG:%s: %s set to %s from tag", envVar, name, tag)
		} else {
			log.Printf("CONFIG:%s: %s not set", envVar, name)
		}
		
		if env == "" && tag != "" {
			env = tag
		}

		field.SetString(env)
	}

	if port := os.Getenv("PORT"); port != "" {
		// If $PORT is set, override LFS_LISTEN. This is useful for deploying to Heroku.
		Config.Listen = "tcp://:" + port
	}
}
