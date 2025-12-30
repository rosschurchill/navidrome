package persistence

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AdvancedSearch", func() {
	Describe("ParseAdvancedSearch", func() {
		It("parses simple field:value patterns", func() {
			result := ParseAdvancedSearch("media_file", "artist:Beatles")
			Expect(result.FullText).To(Equal(""))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("parses multiple field patterns", func() {
			result := ParseAdvancedSearch("media_file", "artist:Beatles year:2020")
			Expect(result.FullText).To(Equal(""))
			Expect(result.Filters).To(HaveLen(2))
		})

		It("preserves remaining text for full-text search", func() {
			result := ParseAdvancedSearch("media_file", "artist:Beatles love me do")
			Expect(result.FullText).To(Equal("love me do"))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("handles quoted values", func() {
			result := ParseAdvancedSearch("media_file", `artist:"The Beatles"`)
			Expect(result.FullText).To(Equal(""))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("handles range patterns", func() {
			result := ParseAdvancedSearch("media_file", "year:2010-2020")
			Expect(result.FullText).To(Equal(""))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("handles plus patterns", func() {
			result := ParseAdvancedSearch("media_file", "rating:4+")
			Expect(result.FullText).To(Equal(""))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("ignores unknown fields", func() {
			result := ParseAdvancedSearch("media_file", "unknown:value artist:Beatles")
			Expect(result.FullText).To(Equal("unknown:value"))
			Expect(result.Filters).To(HaveLen(1))
		})

		It("handles mixed queries", func() {
			result := ParseAdvancedSearch("media_file", "love artist:Beatles year:1960-1970 song")
			Expect(result.FullText).To(Equal("love song"))
			Expect(result.Filters).To(HaveLen(2))
		})
	})

	Describe("buildFilter", func() {
		It("creates LIKE filter for string fields", func() {
			filter := buildFilter("media_file.artist", "Beatles")
			sql, args, err := filter.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("LIKE"))
			Expect(args).To(ContainElement("%Beatles%"))
		})

		It("creates range filter for min-max patterns", func() {
			filter := buildFilter("media_file.year", "2010-2020")
			sql, _, err := filter.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring(">="))
			Expect(sql).To(ContainSubstring("<="))
		})

		It("creates GtOrEq filter for plus patterns", func() {
			filter := buildFilter("COALESCE(annotation.rating, 0)", "4+")
			sql, args, err := filter.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring(">="))
			Expect(args).To(ContainElement(4))
		})

		It("creates boolean filter for true/false values", func() {
			filter := buildFilter("COALESCE(annotation.starred, false)", "true")
			sql, args, err := filter.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("="))
			Expect(args).To(ContainElement(true))
		})
	})
})

func TestAdvancedSearch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Advanced Search Suite")
}
