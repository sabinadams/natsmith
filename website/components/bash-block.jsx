import { codeToHtml } from 'shiki'
import { Pre, Code } from 'nextra/components'

function extractCodeInner(html) {
  const match = html.match(/<code[^>]*>([\s\S]*)<\/code>/)
  return match ? match[1] : html
}

export async function BashBlock({ code }) {
  const html = await codeToHtml(code.trimEnd(), {
    lang: 'bash',
    themes: {
      light: 'github-light',
      dark: 'github-dark',
    },
    defaultColor: false,
    cssVariablePrefix: '--shiki-',
  })

  return (
    <Pre
      data-language="bash"
      data-copy=""
      data-word-wrap=""
      data-pagefind-ignore="all"
      tabIndex={0}
    >
      <Code dangerouslySetInnerHTML={{ __html: extractCodeInner(html) }} />
    </Pre>
  )
}
