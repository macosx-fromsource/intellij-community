package generator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Content", func() {
	var content Content

	Describe("AsMap", func() {
		Context("with a non-empty map", func() {
			BeforeEach(func() {
				content = Content{"key": []byte("value")}
			})

			It("should return the map", func() {
				Expect(content.AsMap()).To(Equal(map[string][]byte{"key": []byte("value")}))
			})
		})

		Context("with an empty map", func() {
			BeforeEach(func() {
				content = Content{}
			})

			It("should empty", func() {
				Expect(content.AsMap()).To(BeEmpty())
			})
		})
	})

	Describe("Get", func() {
		Context("with a non-empty map", func() {
			BeforeEach(func() {
				content = Content{"key": []byte("value")}
			})

			It("should return the value", func() {
				Expect(content.Get("key")).To(Equal([]byte("value")))
			})
		})

		Context("with an empty map", func() {
			BeforeEach(func() {
				content = Content{}
			})

			It("should return the value", func() {
				Expect(content.Get("key")).To(BeNil())
			})
		})
	})

	Describe("GetString", func() {
		Context("with a non-empty map", func() {
			BeforeEach(func() {
				content = Content{"key": []byte("value")}
			})

			It("should return the value", func() {
				Expect(content.GetString("key")).To(Equal("value"))
			})
		})

		Context("with an empty map", func() {
			BeforeEach(func() {
				content = Content{}
			})

			It("should return the value", func() {
				Expect(content.GetString("key")).To(Equal(""))
			})
		})
	})
})
