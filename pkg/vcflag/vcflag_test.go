package vcflag

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestVCFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VCFlag test suite")
}

var _ = Describe("Test GenerateFlags", func() {
	Context("Simple cases", func() {
		Context("simple int", func() {
			It("Should generate flags ok", func() {
				var data int
				command := &cobra.Command{}
				err := GenerateFlags("", "key", data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"key"}))
				Expect(values).To(Equal([]string{"int"}))
			})
		})
		Context("slice of int", func() {
			It("Should generate flags ok", func() {
				var data []int
				command := &cobra.Command{}
				err := GenerateFlags("", "key", data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"key"}))
				Expect(values).To(Equal([]string{"intSlice"}))
			})
		})
		Context("duration", func() {
			It("Should generate flags ok", func() {
				var data = 3 * time.Second
				command := &cobra.Command{}
				err := GenerateFlags("", "key", data, viper.New(), command)
				flags := []string{}
				values := []string{}
				command.Flags().VisitAll(func(pf *pflag.Flag) {
					flags = append(flags, pf.Name)
					values = append(values, pf.Value.Type())
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(flags).To(Equal([]string{"key"}))
				Expect(values).To(Equal([]string{"duration"}))
			})
		})

		Context("struct", func() {
			It("Should generate flags ok", func() {

				type dummyStruct struct {
					A int
					B string
				}

				var data = dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags("", "", data, viper.New(), command)
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

				var data = dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags("", "", data, viper.New(), command)
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

				var data = dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags("", "", data, viper.New(), command)
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

				var data = dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags("", "", data, viper.New(), command)
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

				var data = dummyStruct{}
				command := &cobra.Command{}
				err := GenerateFlags("", "", data, viper.New(), command)
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
