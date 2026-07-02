import nextra from 'nextra'

const basePath = process.env.NEXT_PUBLIC_BASE_PATH || ''

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  basePath,
  assetPrefix: basePath,
  images: {
    unoptimized: true,
  },
  trailingSlash: true,
}

export default withNextra(nextConfig)
