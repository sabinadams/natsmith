import { useMDXComponents as getThemeComponents } from 'nextra-theme-docs'
import {
  CurrentRelease,
  DarwinArm64Download,
  GoInstallPin,
  ReleaseCallout,
} from './components/release-snippets'

const themeComponents = getThemeComponents()

export function useMDXComponents(components) {
  return {
    ...themeComponents,
    CurrentRelease,
    DarwinArm64Download,
    GoInstallPin,
    ReleaseCallout,
    ...components,
  }
}
