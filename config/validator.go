package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nest-egg/ami-replacer/log"
	"golang.org/x/xerrors"
)

//AWSHomeDir get aws config directory.
var AWSHomeDir = func() string {
	var home string
	home = os.Getenv("HOME")
	return filepath.Join(home, ".aws")
}

//ParseRegion parses region.
func ParseRegion(i string) (interface{}, error) {
	if !IsValidRegion(i) {
		return i, xerrors.New("not a valid region")
	}
	return i, nil
}

//IsValidRegion validates given region
func IsValidRegion(i string) bool {
	log.Debug.Println(i)
	reg, _ := regexp.Compile("^(us|eu|ap|sa|ca)\\-\\w+\\-\\d+$")
	return reg.MatchString(i)
}

//IsValidProfile validate given profile.
func IsValidProfile(profile string) bool {
	return stringInSlice(profile, ExistingProfiles())
}

var profileNameRegex = regexp.MustCompile(`\[(.*)\]`)

var awsHomeFunc func() string

//ExistingProfiles returns all profiles in aws config.
func ExistingProfiles() (profiles []string) {
	awsHomeFunc = AWSHomeDir
	awsHome := awsHomeFunc()
	files := []string{filepath.Join(awsHome, "config"), filepath.Join(awsHome, "credentials")}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		out, err := ioutil.ReadFile(f)
		if err != nil {
			continue
		}
		matches := profileNameRegex.FindAllSubmatch(out, -1)
		for _, match := range matches {
			profile := string(match[1])
			profile = strings.TrimSpace(profile)
			profile = strings.TrimPrefix(profile, "profile ")
			profile = strings.TrimSpace(profile)
			if profile != "" {
				profiles = append(profiles, profile)
			}
		}
	}
	return profiles
}

func stringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
