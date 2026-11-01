// Fails when src/api/schema.d.ts is not what `npm run types` would write
// from ../docs/openapi.yaml, so an API change cannot land without its types.
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const committed = 'src/api/schema.d.ts'
const dir = mkdtempSync(join(tmpdir(), 'agc-panel-types-'))
const fresh = join(dir, 'schema.d.ts')

try {
  execFileSync('node_modules/.bin/openapi-typescript', ['../docs/openapi.yaml', '-o', fresh], {
    stdio: ['ignore', 'ignore', 'inherit'],
  })
  if (readFileSync(fresh, 'utf8') !== readFileSync(committed, 'utf8')) {
    console.error(`${committed} is out of date with ../docs/openapi.yaml: run npm run types`)
    process.exitCode = 1
  } else {
    console.log(`${committed} matches ../docs/openapi.yaml`)
  }
} finally {
  rmSync(dir, { recursive: true, force: true })
}
