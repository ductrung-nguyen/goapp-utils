package vcflag

import (
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestVCFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, repoterConfig := GinkgoConfiguration()
	suiteConfig.PollProgressAfter = 1 * time.Second
	repoterConfig.FullTrace = true
	RunSpecs(t, "VCFlag test suite")
}

var _ = Describe("Test GenerateFlags", func() {
	Context("Simple cases", func() {
		Context("simple int", func() {
			It("Should generate flags ok", func() {
				var data int
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{""}))
				Expect(values).To(Equal([]string{"int"}))
			})
		})
		Context("slice of int", func() {
			It("Should generate flags ok", func() {
				var data []int
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{""}))
				Expect(values).To(Equal([]string{"intSlice"}))
			})
		})
		Context("duration", func() {
			It("Should generate flags ok", func() {
				data := 3 * time.Second
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{""}))
				Expect(values).To(Equal([]string{"duration"}))
			})
		})

		Context("struct", func() {
			It("Should generate flags ok", func() {
				type dummyStruct struct {
					A int
					B string
				}

				data := dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"A", "B"}))
				Expect(values).To(Equal([]string{"int", "string"}))
			})
		})
		Context("struct", func() {
			It("Should generate flags ok", func() {
				type nestedStruct struct {
					D []string
					E []int
				}
				type dummyStruct struct {
					A int
					B string
					C nestedStruct
				}

				data := dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"A", "B", "C.D", "C.E"}))
				Expect(values).To(Equal([]string{"int", "string", "stringSlice", "intSlice"}))
			})
		})

		Context("simple struct with unexported field", func() {
			It("Should generate flags ok", func() {
				type dummyStruct struct {
					a int
					b string
				}

				data := dummyStruct{a: 1, b: "str"}
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"a", "b"}))
				Expect(values).To(Equal([]string{"int", "string"}))
			})
		})

		Context("nested struct with unexported field", func() {
			It("Should generate flags ok", func() {
				type nestedStruct struct {
					d []string
					e []int `pflag:"-"`
				}
				type dummyStruct struct {
					a int
					b string
					c nestedStruct
				}

				data := dummyStruct{a: 1, b: "str", c: nestedStruct{d: []string{"str 1", "str 2"}, e: []int{}}}
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"a", "b", "c.d"}))
				Expect(values).To(Equal([]string{"int", "string", "stringSlice"}))
			})
		})

		Context("nested struct with unexported field and tag mapstructure", func() {
			It("Should generate flags ok", func() {
				type nestedStruct struct {
					d []string
					e []int `pflag:"-"`
				}
				type dummyStruct struct {
					a int
					b string `mapstructure:"-"`
					c nestedStruct
				}

				data := dummyStruct{a: 1, b: "str", c: nestedStruct{d: []string{"str 1", "str 2"}, e: []int{}}}
				command := &cobra.Command{}
				err := GenerateFlags(data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"a", "c.d"}))
				Expect(values).To(Equal([]string{"int", "stringSlice"}))
			})
		})
	})
})

func GetFakeLoggerWithGinkgo() logr.Logger {
	return funcr.New(func(prefix, args string) {
		GinkgoWriter.Printf(prefix, args)
		// fmt.Printf(prefix+"\n", args)
	}, funcr.Options{})
}

var _ = Describe("Test BindEnvVarsToFlags", func() {
	Context("config is an nested object", func() {
		It("it should bind the flags with env vars correctly", func() {
			viperObj := viper.GetViper()
			cmd := &cobra.Command{
				Use: "test",
			}
			logger := GetFakeLoggerWithGinkgo()

			type nestedStruct struct {
				d []string
				e []int `pflag:"-"`
			}
			type dummyStruct struct {
				a int    `pflag:"a" usage:"a simple integer"`
				b string `mapstructure:"-"`
				c nestedStruct
				f bool `pflag:"f" usage:" force or not"`
			}

			data := dummyStruct{
				a: 1, b: "str",
				c: nestedStruct{
					d: []string{"str 1", "str 2"},
					e: []int{},
				},
				f: true,
			}

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "TEST", &logger)

			flags := []string{}
			valueTypes := []string{}
			cmd.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				valueTypes = append(valueTypes, pf.Value.Type())
				Expect(pf.Usage).To(ContainSubstring("Overrided by Env Var "))
				switch pf.Name {
				case "a":
					Expect(pf.Usage).To(ContainSubstring("a simple integer"))
				case "c":
					Expect(pf.Usage).To(ContainSubstring("force or not"))
				}
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"a", "c.d", "f"}))
			Expect(valueTypes).To(Equal([]string{"int", "stringSlice", "bool"}))

			// viperObj.Set("c.d", []string{"splunk"})

			// unmarshaledData := &dummyStruct{}
			// err = viperObj.Unmarshal(unmarshaledData)
			// Expect(err).NotTo(HaveOccurred())
			// Expect(unmarshaledData.c.d).To(Equal([]string{"str 1", "str 2"}))
		})
	})
})

var _ = Describe("Test InitConfigReader", func() {
	Context("config is an nested object", func() {
		It("it should return the configuration object correctly", func() {
			viperObj := viper.GetViper()
			cmd := &cobra.Command{
				Use: "test",
			}
			logger := GetFakeLoggerWithGinkgo()

			type nestedStruct struct {
				D []string `yaml:"d"`
				E []int    `yaml:"e"`
			}
			type dummyStruct struct {
				A int          `yaml:"a" pflag:"a; a simple integer"`
				B string       `yaml:"b" mapstructure:"-"`
				C nestedStruct `yaml:"c"`
			}

			data := dummyStruct{}

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "TEST", &logger)

			configStr := `
a: 10
b: a simple b string
c:
  d:
    - str 1
    - str 2
  e:
    - 1
    - 2
    - 3
`
			configFile, err := os.CreateTemp(".", "*.yaml")
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := configFile.Close(); err != nil {
					panic(err)
				}
			}()
			configFilename := configFile.Name()[2:] // the file name is in form of ./name.yaml, we want to remove ./
			defer func() {
				if err := os.Remove(configFilename); err != nil {
					panic(err)
				}
			}()
			err = os.WriteFile(configFilename, []byte(configStr), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = InitConfigReader(viperObj, cmd, configFilename, "", "", []string{}, "TEST", &logger, true)
			Expect(err).NotTo(HaveOccurred())

			unmarshaledData := &dummyStruct{}
			err = viperObj.Unmarshal(unmarshaledData)
			Expect(err).NotTo(HaveOccurred())
			Expect(unmarshaledData.C.D).To(Equal([]string{"str 1", "str 2"}))
			Expect(unmarshaledData.C.E).To(Equal([]int{1, 2, 3}))
		})
	})

	Context("config is an nested object + not bind env vars to flags", func() {
		var configStr string
		var cmd *cobra.Command
		logger := GetFakeLoggerWithGinkgo()

		type nestedStruct struct {
			D []string `yaml:"d"`
			E []int    `yaml:"e"`
		}
		type dummyStruct struct {
			A int          `yaml:"a" pflag:"a" usage:"a simple integer"`
			B string       `yaml:"b" mapstructure:"-"`
			C nestedStruct `yaml:"c"`
		}
		configStr = `
a: 10
b: a simple b string
c:
  d:
    - str 1
    - str 2
  e:
    - 1
    - 2
    - 3
`

		data := dummyStruct{}

		BeforeEach(func() {
			cmd = &cobra.Command{
				Use: "test",
			}
		})

		It("should return the configuration object with some overrides from env variables", func() {
			GinkgoT().Setenv("TEST_A", "20")
			viperObj := viper.GetViper()

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "TEST", &logger)

			configFile, err := os.CreateTemp(".", "*.yaml")
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := configFile.Close(); err != nil {
					panic(err)
				}
			}()
			configFilename := configFile.Name()[2:] // the file name is in form of ./name.yaml, we want to remove ./
			defer func() {
				if err := os.Remove(configFilename); err != nil {
					panic(err)
				}
			}()
			err = os.WriteFile(configFilename, []byte(configStr), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = InitConfigReader(viperObj, cmd, configFilename, "", "", []string{}, "TEST", &logger, false)
			Expect(err).NotTo(HaveOccurred())

			unmarshaledData := &dummyStruct{}
			err = viperObj.Unmarshal(unmarshaledData)
			Expect(err).NotTo(HaveOccurred())
			Expect(unmarshaledData.A).To(Equal(20))
			Expect(unmarshaledData.C.D).To(Equal([]string{"str 1", "str 2"}))
			Expect(unmarshaledData.C.E).To(Equal([]int{1, 2, 3}))
		})

		It("should return the configuration object W/O some overrides from env variables", func() {
			viperObj := viper.GetViper()

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "TEST", &logger)

			configFile, err := os.CreateTemp(".", "*.yaml")
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := configFile.Close(); err != nil {
					panic(err)
				}
			}()
			configFilename := configFile.Name()[2:] // the file name is in form of ./name.yaml, we want to remove ./
			defer func() {
				if err := os.Remove(configFilename); err != nil {
					panic(err)
				}
			}()
			err = os.WriteFile(configFilename, []byte(configStr), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = InitConfigReader(viperObj, cmd, configFilename, "", "", []string{}, "TEST", &logger, false)
			Expect(err).NotTo(HaveOccurred())

			unmarshaledData := &dummyStruct{}
			err = viperObj.Unmarshal(unmarshaledData)
			Expect(err).NotTo(HaveOccurred())
			Expect(unmarshaledData.A).To(Equal(10))
			Expect(unmarshaledData.C.D).To(Equal([]string{"str 1", "str 2"}))
			Expect(unmarshaledData.C.E).To(Equal([]int{1, 2, 3}))
		})
	})
})
