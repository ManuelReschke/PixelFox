package utils

import (
	"regexp"
	"strings"
)

// ProcessHTMLContent adds Tailwind classes to HTML elements
func ProcessHTMLContent(content string) string {
	// Map of HTML tags to Tailwind classes
	replacements := map[string]string{
		`<h1([^>]*)>`:         `<h1$1 class="text-4xl font-bold mb-4 mt-6">`,
		`<h2([^>]*)>`:         `<h2$1 class="text-3xl font-bold mb-3 mt-5">`,
		`<h3([^>]*)>`:         `<h3$1 class="text-2xl font-bold mb-2 mt-4">`,
		`<h4([^>]*)>`:         `<h4$1 class="text-xl font-bold mb-2 mt-3">`,
		`<h5([^>]*)>`:         `<h5$1 class="text-lg font-bold mb-1 mt-2">`,
		`<h6([^>]*)>`:         `<h6$1 class="text-base font-bold mb-1 mt-2">`,
		`<p([^>]*)>`:          `<p$1 class="mb-4 text-base-content leading-relaxed">`,
		`<ul([^>]*)>`:         `<ul$1 class="list-disc list-inside mb-4 ml-4 space-y-2">`,
		`<ol([^>]*)>`:         `<ol$1 class="list-decimal list-inside mb-4 ml-4 space-y-2">`,
		`<li([^>]*)>`:         `<li$1 class="text-base-content">`,
		`<blockquote([^>]*)>`: `<blockquote$1 class="border-l-4 border-primary pl-4 italic mb-4 text-base-content/80">`,
		`<table([^>]*)>`:      `<table$1 class="table table-bordered w-full mb-4">`,
		`<code([^>]*)>`:       `<code$1 class="bg-base-200 px-2 py-1 rounded text-sm font-mono">`,
		`<pre([^>]*)>`:        `<pre$1 class="bg-base-200 p-4 rounded-lg mb-4 overflow-x-auto">`,
		`<a([^>]*)>`:          `<a$1 class="link link-primary">`,
		`<strong([^>]*)>`:     `<strong$1 class="font-bold">`,
		`<em([^>]*)>`:         `<em$1 class="italic">`,
	}

	processedContent := content

	for pattern, replacement := range replacements {
		// Only replace if the element doesn't already have a class attribute
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(processedContent, -1)

		for _, match := range matches {
			if len(match) > 1 && !strings.Contains(match[1], "class=") {
				processedContent = strings.Replace(processedContent, match[0], replacement, 1)
			}
		}
	}

	return processedContent
}
