import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import 'nextra-theme-docs/style.css'

export const metadata = {
  title: {
    default: 'natsmith',
    template: '%s – natsmith',
  },
  description:
    'CLI tooling for NATS and JetStream.',
}

export default async function RootLayout({ children }) {
  const navbar = (
    <Navbar
      logo={<strong>natsmith</strong>}
      projectLink="https://github.com/sabinadams/natsmith"
    />
  )
  const pageMap = await getPageMap()

  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head />
      <body>
        <Layout
          navbar={navbar}
          pageMap={pageMap}
          docsRepositoryBase="https://github.com/sabinadams/natsmith/tree/main/website/content"
          footer={
            <Footer>
              MIT {new Date().getFullYear()} © natsmith ·{' '}
              <a href="https://github.com/sabinadams/natsmith">GitHub</a>
            </Footer>
          }
          editLink="Edit this page on GitHub"
          sidebar={{ defaultMenuCollapseLevel: 1 }}
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
