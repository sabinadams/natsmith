import { Pre, Code } from 'nextra/components'
import { version, tag } from '../lib/version'

export function CurrentRelease() {
  return <code>{tag}</code>
}

export function ReleaseCallout() {
  return (
    <p>
      <strong>Current release:</strong> <CurrentRelease />
    </p>
  )
}

export function GoInstallPin() {
  return (
    <Pre data-language="bash" data-copy="">
      <Code>{`go install github.com/sabinadams/natsmith/cmd/natsmith@${tag}`}</Code>
    </Pre>
  )
}

export function DarwinArm64Download() {
  return (
    <Pre data-language="bash" data-copy="">
      <Code>{`VERSION=${version}
curl -LO "https://github.com/sabinadams/natsmith/releases/download/v\${VERSION}/natsmith_\${VERSION}_darwin_arm64.tar.gz"
tar xzf "natsmith_\${VERSION}_darwin_arm64.tar.gz"
chmod +x natsmith
sudo mv natsmith /usr/local/bin/`}</Code>
    </Pre>
  )
}
