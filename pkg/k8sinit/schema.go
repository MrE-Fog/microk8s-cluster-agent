package k8sinit

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/version"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

var (
	minimumConfigFileVersionRequired  = version.MustParseSemantic("0.1.0")
	maximumConfigFileVersionSupported = version.MustParseSemantic("0.1.0")

	// errEmptyConfig is an ignorable error when parsing empty YAML documents
	errEmptyConfig = fmt.Errorf("empty configuration object")
)

// AddonConfiguration specifies an addon to be enabled or disabled.
type AddonConfiguration struct {
	// Name of the addon to configure.
	Name string `yaml:"name"`

	// Disable the addon.
	Disable bool `yaml:"disable"`

	// Arguments is optional arguments passed to the addon enable or disable operation.
	Arguments []string `yaml:"args"`
}

// MultiPartConfiguration is a configuration split into multiple parts.
type MultiPartConfiguration struct {
	// Parts are configuration objects that are meant to be applied in order.
	Parts []*Configuration
}

// Configuration is the top-level definition for MicroK8s configuration files.
type Configuration struct {
	// Version is the semantic version of the configuration file format.
	Version string `yaml:"version"`

	// Addons is a list of addons to enable and/or disable.
	Addons []AddonConfiguration `yaml:"addons"`

	// ExtraKubeletArgs is a list of extra arguments to add to the local node kubelet.
	// Set a value to null to remove it from the arguments.
	ExtraKubeletArgs map[string]*string `yaml:"extraKubeletArgs"`

	// ExtraKubeAPIServerArgs is a list of extra arguments to add to the local node kube-apiserver.
	// Set a value to null to remove it from the arguments.
	ExtraKubeAPIServerArgs map[string]*string `yaml:"extraKubeAPIServerArgs"`

	// ExtraSANs are a list of extra Subject Alternate Names to add to the local API server.
	ExtraSANs []string `yaml:"extraSANs"`
}

// ParseConfiguration tries to parse a Configuration object from YAML data.
func ParseConfiguration(input []byte) (*Configuration, error) {
	c := &Configuration{}

	if strictParseErr := yaml.UnmarshalStrict(input, c); strictParseErr != nil {
		// If non-strict parsing also fails, then raise the error
		if err := yaml.Unmarshal(input, c); err != nil {
			return nil, fmt.Errorf("could not parse configuration: %w", err)
		}

		log.Printf("WARNING: configuration may contain unknown fields (error was %q).", strictParseErr)
		log.Printf("Any unknown fields will be ignored")
	}

	if c.isZero() {
		return nil, errEmptyConfig
	}

	v, err := version.ParseSemantic(c.Version)
	switch {
	case err != nil:
		return nil, fmt.Errorf("could not parse config file version %q: %w", c.Version, err)
	case maximumConfigFileVersionSupported.LessThan(v):
		return nil, fmt.Errorf("config file version is %v but the maximum version supported is %v", c.Version, maximumConfigFileVersionSupported)
	case v.LessThan(minimumConfigFileVersionRequired):
		return nil, fmt.Errorf("config file version is %v but the minimum version required is %v", c.Version, minimumConfigFileVersionRequired)
	}

	return c, nil
}

// ParseMultiPartConfiguration parses a multiple YAML configuration objects into a MultiPartConfiguration.
func ParseMultiPartConfiguration(b []byte) (MultiPartConfiguration, error) {
	reader := k8syaml.NewYAMLReader(bufio.NewReader(bytes.NewBuffer(b)))

	cfg := MultiPartConfiguration{}
	for {
		doc, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			} else if err != nil {
				return MultiPartConfiguration{}, err
			}
		}

		part, err := ParseConfiguration(doc)
		if err != nil {
			if errors.Is(err, errEmptyConfig) {
				continue
			}
			return MultiPartConfiguration{}, err
		}
		cfg.Parts = append(cfg.Parts, part)
	}

	return cfg, nil
}

// isZero returns true if all configuration values are zero/empty.
// NOTE(neoaggelos): this needs to be updated when new fields are added to the Configuration struct.
func (c *Configuration) isZero() bool {
	return c.Version == "" &&
		len(c.Addons) == 0 &&
		len(c.ExtraKubeAPIServerArgs) == 0 &&
		len(c.ExtraKubeletArgs) == 0 &&
		len(c.ExtraSANs) == 0
}
