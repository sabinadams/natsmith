import { Callout } from 'nextra/components'
import { version, tag } from '../lib/version'
import { BashBlock } from './bash-block'

export function CurrentRelease() {
  return (
    <code className="nextra-code" dir="ltr">
      {tag}
    </code>
  )
}

export function ReleaseCallout() {
  return (
    <Callout type="info" className="x:not-first:mt-[1.25em]">
      <strong>Current release:</strong> <CurrentRelease />
    </Callout>
  )
}

export async function GoInstallPin() {
  return (
    <BashBlock
      code={`go install github.com/sabinadams/natsmith/cmd/natsmith@${tag}`}
    />
  )
}

export async function DarwinArm64Download() {
  return (
    <BashBlock
      code={`VERSION=${version}
curl -LO "https://github.com/sabinadams/natsmith/releases/download/v\${VERSION}/natsmith_\${VERSION}_darwin_arm64.tar.gz"
tar xzf "natsmith_\${VERSION}_darwin_arm64.tar.gz"
chmod +x natsmith
sudo mv natsmith /usr/local/bin/`}
    />
  )
}
