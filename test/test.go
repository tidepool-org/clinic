package test

import (
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"regexp"
	"runtime"
	"testing"
)


func Test(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, getCallerPackage())
}

func getCallerPackage() string {
	var callerPackage string
	if matches := callerPackageRegexp.FindStringSubmatch(getFrameName(3)); matches != nil {
		callerPackage = matches[1]
	}
	return callerPackage
}

func getFrameName(frame int) string {
	var frameName string
	if pc, _, _, ok := runtime.Caller(frame); ok {
		frameName = runtime.FuncForPC(pc).Name()
	}
	return frameName
}

var callerPackageRegexp = regexp.MustCompile("^(.+?)(?:_test)[^/]+$")
