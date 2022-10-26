package utils

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test FileExists", func() {
	Context("File exist", func() {
		It("should return true", func() {
			f, err := os.CreateTemp(".", "*.txt")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			filename := f.Name()
			defer os.Remove(filename)

			Expect(FileExists(filename)).To(BeTrue())
		})
	})

	Context("File does not exist", func() {
		It("should return false", func() {
			f, err := os.CreateTemp(".", "*.txt")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			filename := f.Name()
			defer os.Remove(filename)

			Expect(FileExists(filename + "-wrong")).To(BeFalse())
		})
	})
})

var _ = Describe("Test FolderExists", func() {
	Context("Folder exists", func() {
		It("should return True", func() {
			folderName, err := os.MkdirTemp(".", "*")
			if err != nil {
				panic(err)
			}

			defer os.Remove(folderName)

			FolderExists(folderName)
			Expect(FolderExists(folderName)).To(BeTrue())
		})

		It("should return False", func() {
			folderName, err := os.MkdirTemp(".", "*")
			if err != nil {
				panic(err)
			}

			defer os.Remove(folderName)

			FolderExists(folderName)
			Expect(FolderExists(folderName + "-wrong")).To(BeFalse())
		})
	})

})

var _ = Describe("Test GetCurrentExecPath", func() {
	It("should get current path ok", func() {
		ef := GetCurrentExecPath()
		Expect(ef).NotTo(BeEmpty())
	})
})

var _ = DescribeTable(
	"Test GetParams",
	func(reg string, str string, expected map[string]string) {
		Expect(GetParams(reg, str)).To(BeEquivalentTo(expected))
	},
	Entry(
		"test 1",
		`(?P<Year>\d{4})-(?P<Month>\d{2})-(?P<Day>\d{2})`,
		"2020-10-13",
		map[string]string{
			"Day": "13", "Month": "10", "Year": "2020",
		},
	),
	Entry(
		"test 2",
		`(?P<else>else|\s)\s+(?P<if>if \((?P<contidion>.*)\)\s*|){`,
		` for (uint8_t IdxChar = 0; IdxChar < 5; ++IdxChar) {
		if (FlightStr[IdxChar] == ' ') {`,
		map[string]string{
			"contidion": "FlightStr[IdxChar] == ' '",
			"else":      "\n",
			"if":        "if (FlightStr[IdxChar] == ' ') ",
		},
	),
	Entry(
		"test 3",
		`(?P<else>else|\s)\s+(?P<if>if \((?P<contidion>.*)\)\s*|){`,
		" } else if ((!isdigit((unsigned int)FlightStr[IdxChar])) && (FlightStr[IdxChar] != '\\0')) {",
		map[string]string{
			"contidion": "(!isdigit((unsigned int)FlightStr[IdxChar])) && (FlightStr[IdxChar] != '\\0')",
			"else":      "else",
			"if":        "if ((!isdigit((unsigned int)FlightStr[IdxChar])) && (FlightStr[IdxChar] != '\\0')) ",
		},
	),
	Entry(
		"test case 4",
		`(?P<else>else|\s)\s+(?P<if>if \((?P<contidion>.*)\)\s*|){`,
		`} else {
					m = 30;
				}`,
		map[string]string{
			"contidion": "",
			"else":      "else",
			"if":        "",
		},
	),
)
