package version

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	// REGEXVERSIONKIND represents versions as a simple string
	REGEXVERSIONKIND string = "regex"
	// SEMVERVERSIONKIND represents versions as a semantic versioning type
	SEMVERVERSIONKIND string = "semver"
	// LATESTVERSIONKIND specifies that we are looking for the latest version of an array
	LATESTVERSIONKIND string = "latest"
)

var (
	// SupportedKind holds a list of supported version kind
	SupportedKind []string = []string{
		REGEXVERSIONKIND,
		SEMVERVERSIONKIND,
		LATESTVERSIONKIND,
	}
)

// Filter defines parameters to apply different kind of version matching based on a list of versions
type Filter struct {
	// Specifies the version kind such as semver, regex, or latest
	Kind string `yaml:",omitempty"`
	// Specifies the version pattern according the version kind
	Pattern string `yaml:",omitempty"`
	// Strict enforce strict versioning rule. Only used for semantic versioning at this time
	Strict bool `yaml:",omitempty"`
}

// Init returns a new (copy) valid instanciated filter
func (f Filter) Init() (Filter, error) {
	// Set default kind value to "latest"
	if len(f.Kind) == 0 {
		f.Kind = LATESTVERSIONKIND
	}

	// Set default pattern value based on kind
	if f.Kind == LATESTVERSIONKIND && len(f.Pattern) == 0 {
		f.Pattern = LATESTVERSIONKIND
	} else if f.Kind == SEMVERVERSIONKIND && len(f.Pattern) == 0 {
		f.Pattern = "*"
	} else if f.Kind == REGEXVERSIONKIND && len(f.Pattern) == 0 {
		f.Pattern = ".*"
	}

	return f, f.Validate()
}

// Validate tests if our filter contains valid parameters
func (f Filter) Validate() error {
	ok := false

	for id := range SupportedKind {
		if SupportedKind[id] == f.Kind {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("unsupported version kind %q", f.Kind)
	}
	return nil
}

// Search returns a value matching pattern
func (f *Filter) Search(versions []string) (Version, error) {

	logrus.Infof("Searching for version matching pattern %q", f.Pattern)

	foundVersion := Version{}

	switch f.Kind {
	case LATESTVERSIONKIND:
		if f.Pattern == LATESTVERSIONKIND {
			foundVersion.ParsedVersion = versions[len(versions)-1]
			foundVersion.OriginalVersion = foundVersion.ParsedVersion
			return foundVersion, nil
		}
		// Search for simple text matching
		for i := len(versions) - 1; i >= 0; i-- {
			if strings.Compare(f.Pattern, versions[i]) == 0 {
				foundVersion.ParsedVersion = versions[i]
				foundVersion.OriginalVersion = versions[i]
				return foundVersion, nil
			}
		}
	case REGEXVERSIONKIND:
		re, err := regexp.Compile(f.Pattern)
		if err != nil {
			return foundVersion, err
		}

		// Parse version in by date publishing
		// Oldest version appears first in array
		for i := len(versions) - 1; i >= 0; i-- {
			v := versions[i]
			if re.MatchString(v) {
				foundVersion.ParsedVersion = v
				foundVersion.OriginalVersion = v
				return foundVersion, nil
			}
		}
	case SEMVERVERSIONKIND:
		s := Semver{
			Constraint: f.Pattern,
			Strict:     f.Strict,
		}

		err := s.Search(versions)
		if err != nil {
			return foundVersion, err
		}

		return s.FoundVersion, nil
	default:
		return foundVersion, fmt.Errorf("unsupported version kind %q with pattern %q", f.Kind, f.Pattern)
	}

	return foundVersion, fmt.Errorf("no version found matching pattern %q", f.Pattern)
}

// IsZero return true if filter is not initialized
func (f Filter) IsZero() bool {
	var empty Filter
	return empty == f
}
