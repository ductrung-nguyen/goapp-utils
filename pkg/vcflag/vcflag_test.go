package vcflag

import (
	"errors"
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

var _ = Describe("Test Pointer Support", func() {
	Context("Pointer to primitive types", func() {
		It("Should generate flags correctly for nil pointer fields", func() {
			type dummyStruct struct {
				A *int
				B *string
				C *bool
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
			Expect(flags).To(Equal([]string{"A", "B", "C"}))
			Expect(values).To(Equal([]string{"int", "string", "bool"}))
		})

		It("Should generate flags correctly for non-nil pointer fields", func() {
			intVal := 42
			strVal := "test"
			boolVal := true
			type dummyStruct struct {
				A *int
				B *string
				C *bool
			}

			data := dummyStruct{
				A: &intVal,
				B: &strVal,
				C: &boolVal,
			}
			command := &cobra.Command{}
			err := GenerateFlags(data, viper.New(), command)
			flags := []string{}
			values := []string{}
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				values = append(values, pf.Value.Type())
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"A", "B", "C"}))
			Expect(values).To(Equal([]string{"int", "string", "bool"}))
		})
	})

	Context("Pointer to time.Duration", func() {
		It("Should generate flags correctly for nil duration pointer", func() {
			type dummyStruct struct {
				Timeout *time.Duration
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
			Expect(flags).To(Equal([]string{"Timeout"}))
			Expect(values).To(Equal([]string{"duration"}))
		})

		It("Should generate flags correctly for non-nil duration pointer", func() {
			timeout := 30 * time.Second
			type dummyStruct struct {
				Timeout *time.Duration
			}

			data := dummyStruct{
				Timeout: &timeout,
			}
			command := &cobra.Command{}
			err := GenerateFlags(data, viper.New(), command)
			flags := []string{}
			values := []string{}
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				values = append(values, pf.Value.Type())
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"Timeout"}))
			Expect(values).To(Equal([]string{"duration"}))
		})
	})

	Context("Pointer to struct", func() {
		It("Should generate flags correctly for nil struct pointer", func() {
			type NestedStruct struct {
				D string
				E int
			}
			type dummyStruct struct {
				A int
				B *NestedStruct
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
			Expect(flags).To(Equal([]string{"A", "B.D", "B.E"}))
			Expect(values).To(Equal([]string{"int", "string", "int"}))
		})

		It("Should generate flags correctly for non-nil struct pointer", func() {
			type NestedStruct struct {
				D string
				E int
			}
			type dummyStruct struct {
				A int
				B *NestedStruct
			}

			data := dummyStruct{
				A: 100,
				B: &NestedStruct{
					D: "nested",
					E: 200,
				},
			}
			command := &cobra.Command{}
			err := GenerateFlags(data, viper.New(), command)
			flags := []string{}
			values := []string{}
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				values = append(values, pf.Value.Type())
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"A", "B.D", "B.E"}))
			Expect(values).To(Equal([]string{"int", "string", "int"}))
		})
	})

	Context("Mixed pointer and non-pointer fields", func() {
		It("Should generate flags correctly for mixed struct", func() {
			type NestedStruct struct {
				X []string
				Y *int
			}
			type dummyStruct struct {
				A int
				B *string
				C *NestedStruct
				D bool
				E *time.Duration
			}

			strVal := "pointer string"
			data := dummyStruct{
				A: 10,
				B: &strVal,
				C: nil, // nil pointer to struct
				D: true,
				E: nil, // nil pointer to duration
			}
			command := &cobra.Command{}
			err := GenerateFlags(data, viper.New(), command)
			flags := []string{}
			values := []string{}
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				values = append(values, pf.Value.Type())
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"A", "B", "C.X", "C.Y", "D", "E"}))
			Expect(values).To(Equal([]string{"int", "string", "stringSlice", "int", "bool", "duration"}))
		})
	})

	Context("Environment variable binding with pointers", func() {
		It("Should bind env vars to pointer fields correctly", func() {
			viperObj := viper.New()
			cmd := &cobra.Command{
				Use: "test",
			}
			logger := GetFakeLoggerWithGinkgo()

			type dummyStruct struct {
				A *int    `pflag:"a" usage:"pointer to int"`
				B *string `pflag:"b" usage:"pointer to string"`
				C *bool   `pflag:"c" usage:"pointer to bool"`
			}

			data := dummyStruct{}

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "PTRTEST", &logger)

			flags := []string{}
			valueTypes := []string{}
			cmd.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				valueTypes = append(valueTypes, pf.Value.Type())
				Expect(pf.Usage).To(ContainSubstring("Overrided by Env Var "))
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(Equal([]string{"a", "b", "c"}))
			Expect(valueTypes).To(Equal([]string{"int", "string", "bool"}))
		})
	})

	Context("Viper unmarshal with pointer fields", func() {
		It("Should unmarshal values into pointer fields correctly", func() {
			viperObj := viper.New()
			cmd := &cobra.Command{
				Use: "test",
			}
			logger := GetFakeLoggerWithGinkgo()

			type NestedStruct struct {
				X []string `yaml:"x"`
				Y *int     `yaml:"y"`
			}
			type dummyStruct struct {
				A *int           `yaml:"a" pflag:"a" usage:"pointer to int"`
				B *string        `yaml:"b" pflag:"b" usage:"pointer to string"`
				C *NestedStruct  `yaml:"c"`
				D *time.Duration `yaml:"d"`
			}

			data := dummyStruct{}

			err := GenerateFlags(data, viperObj, cmd)
			Expect(err).NotTo(HaveOccurred())
			BindEnvVarsToFlags(viperObj, cmd, "PTRTEST", &logger)

			configStr := `
a: 100
b: test string
c:
  x:
    - item1
    - item2
  y: 500
d: 1m30s
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

			err = InitConfigReader(viperObj, cmd, configFilename, "", "", []string{}, "PTRTEST", &logger, true)
			Expect(err).NotTo(HaveOccurred())

			unmarshaledData := &dummyStruct{}
			err = viperObj.Unmarshal(unmarshaledData)
			Expect(err).NotTo(HaveOccurred())
			Expect(unmarshaledData.A).NotTo(BeNil())
			Expect(*unmarshaledData.A).To(Equal(100))
			Expect(unmarshaledData.B).NotTo(BeNil())
			Expect(*unmarshaledData.B).To(Equal("test string"))
			Expect(unmarshaledData.C).NotTo(BeNil())
			Expect(unmarshaledData.C.X).To(Equal([]string{"item1", "item2"}))
			Expect(unmarshaledData.C.Y).NotTo(BeNil())
			Expect(*unmarshaledData.C.Y).To(Equal(500))
			Expect(unmarshaledData.D).NotTo(BeNil())
			Expect(*unmarshaledData.D).To(Equal(90 * time.Second))
		})
	})

	Context("Pointer to slice types", func() {
		It("Should generate flags correctly for pointer to slices", func() {
			type dummyStruct struct {
				A *[]int
				B *[]string
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
			Expect(values).To(Equal([]string{"intSlice", "stringSlice"}))
		})

		It("Should generate flags correctly for non-nil pointer to slices", func() {
			intSlice := []int{1, 2, 3}
			strSlice := []string{"a", "b", "c"}
			type dummyStruct struct {
				A *[]int
				B *[]string
			}

			data := dummyStruct{
				A: &intSlice,
				B: &strSlice,
			}
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
			Expect(values).To(Equal([]string{"intSlice", "stringSlice"}))
		})
	})

	Context("Deeply nested pointers", func() {
		It("Should generate flags correctly for deeply nested struct pointers", func() {
			type Level3 struct {
				Z string
			}
			type Level2 struct {
				Y  int
				L3 *Level3
			}
			type Level1 struct {
				X  string
				L2 *Level2
			}
			type dummyStruct struct {
				A  int
				L1 *Level1
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
			Expect(flags).To(ConsistOf([]string{"A", "L1.X", "L1.L2.Y", "L1.L2.L3.Z"}))
			Expect(values).To(ConsistOf([]string{"int", "string", "int", "string"}))
		})
	})

	Context("Pointer fields with tags", func() {
		It("Should respect pflag and mapstructure tags for pointer fields", func() {
			type dummyStruct struct {
				A *int    `pflag:"custom-a" usage:"custom int pointer"`
				B *string `mapstructure:"-"`
				C *bool   `pflag:"-"`
				D *int
			}

			data := dummyStruct{}
			command := &cobra.Command{}
			err := GenerateFlags(data, viper.New(), command)
			flags := []string{}
			values := []string{}
			usages := []string{}
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				flags = append(flags, pf.Name)
				values = append(values, pf.Value.Type())
				usages = append(usages, pf.Usage)
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(flags).To(ConsistOf([]string{"custom-a", "D"}))
			Expect(values).To(ConsistOf([]string{"int", "int"}))
			// Find the custom-a flag and check its usage
			customAUsage := ""
			command.Flags().VisitAll(func(pf *pflag.Flag) {
				if pf.Name == "custom-a" {
					customAUsage = pf.Usage
				}
			})
			Expect(customAUsage).To(Equal("custom int pointer"))
		})
	})
})

var _ = Describe("CaptureMetadata", func() {
	It("captures local generated flags after parsing and returns immutable copies", func() {
		type config struct {
			Port int      `pflag:"port" usage:"listen port"`
			Name string   `pflag:"name"`
			Tags []string `pflag:"tags"`
		}
		cmd := &cobra.Command{Use: "test"}
		v := viper.New()
		Expect(GenerateFlags(config{}, v, cmd)).To(Succeed())
		Expect(cmd.Flags().Parse([]string{"--port", "8080", "--tags", "a,b"})).To(Succeed())
		Expect(BindEnvVarsToFlagsLocal(v, cmd, "APP", nil, []string{"port", "name", "tags"})).To(Succeed())

		metadata, err := CaptureMetadata(cmd)
		Expect(err).NotTo(HaveOccurred())
		fields := metadata.Fields()
		Expect(fields).To(HaveLen(3))
		Expect(fields[0].Name).To(Equal("name"))
		Expect(fields[1].Value.Kind).To(Equal(ValueKindInt))
		Expect(fields[1].Value.Changed).To(BeTrue())
		Expect(fields[1].Value.DefValue).To(Equal("0"))
		Expect(fields[2].Value.Kind).To(Equal(ValueKindStringSlice))
		Expect(fields[1].Env.Name).To(Equal("APP_PORT"))

		fields[0].Value.String = "mutated"
		Expect(metadata.Fields()[0].Value.String).NotTo(Equal("mutated"))
	})

	It("rejects unsupported custom values with a typed error", func() {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().Var(&testFlagValue{}, "custom", "custom")
		_, err := CaptureMetadata(cmd)
		var unsupported *UnsupportedValueError
		Expect(err).To(MatchError(ContainSubstring("custom")))
		Expect(errors.As(err, &unsupported)).To(BeTrue())
	})
})

type testFlagValue struct{}

func (*testFlagValue) String() string   { return "" }
func (*testFlagValue) Set(string) error { return nil }
func (*testFlagValue) Type() string     { return "custom" }
