import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import mk from '@traptitech/markdown-it-katex'

const md = new MarkdownIt({
  html: true,
  highlight: (str, lang) => {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(str, { language: lang }).value
      } catch (_) {}
    }
    return ''
  }
}).use(mk)

export function renderMarkdown(text) {
  if (!text) return ''
  return md.render(text)
}
