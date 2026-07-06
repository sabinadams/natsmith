import { GitHubNoteIcon } from 'nextra/icons'
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
    <div className="nextra-callout x:not-first:mt-[1.25em] x:flex x:items-center x:gap-3 x:rounded-lg x:border x:px-4 x:py-3 x:bg-blue-100 x:dark:bg-blue-900/30 x:text-blue-700 x:dark:text-blue-400 x:border-blue-700 x:dark:border-blue-600">
      <GitHubNoteIcon height="1em" className="x:shrink-0" />
      <p className="x:m-0 x:leading-normal">
        <strong>Current release:</strong> <CurrentRelease />
      </p>
    </div>
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
