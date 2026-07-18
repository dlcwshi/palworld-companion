import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'

const frontendRoot = resolve(fileURLToPath(new URL('..', import.meta.url)))
const schemaPath = resolve(frontendRoot, 'schemas/breeding-data.schema.json')

export async function validateBreedingData(data) {
  const schema = JSON.parse(await readFile(schemaPath, 'utf8'))
  const ajv = new Ajv2020({ allErrors: true, strict: true })
  addFormats(ajv)
  const validate = ajv.compile(schema)
  if (!validate(data)) throw new Error(ajv.errorsText(validate.errors, { separator: '\n' }))

  const ids = new Set()
  let priorPal = null
  for (const pal of data.pals) {
    if (ids.has(pal.id)) throw new Error(`duplicate pal id: ${pal.id}`)
    ids.add(pal.id)
    const key = [pal.dexNo, pal.dexSuffix ?? '', pal.id]
    if (priorPal && compareTuple(priorPal, key) >= 0) throw new Error(`pals are not in stable order at ${pal.id}`)
    priorPal = key
  }
  const pairs = new Map()
  let priorCombination = null
  const specialChildren = new Set()
  for (const combination of data.specialCombinations) {
    if (![combination.parentA, combination.parentB, combination.child].every((id) => ids.has(id)))
      throw new Error(`special combination references an unknown id: ${combination.parentA}+${combination.parentB}->${combination.child}`)
    if (combination.parentA > combination.parentB) throw new Error('special combination parents are not normalized')
    const pair = `${combination.parentA}\0${combination.parentAGender}\0${combination.parentB}\0${combination.parentBGender}`
    if (pairs.has(pair)) throw new Error(`${pairs.get(pair) === combination.child ? 'duplicate' : 'conflicting'} special combination: ${combination.parentA}+${combination.parentB}`)
    pairs.set(pair, combination.child)
    specialChildren.add(combination.child)
    const key = [combination.parentA, combination.parentB, combination.parentAGender, combination.parentBGender, combination.child]
    if (priorCombination && compareTuple(priorCombination, key) >= 0) throw new Error('specialCombinations are not in stable order')
    priorCombination = key
  }
  for (const pal of data.pals) {
    if (pal.normalBreedingResult === 'specialOnly' && !specialChildren.has(pal.id)) throw new Error(`${pal.id}: specialOnly without a special combination`)
    if (pal.normalBreedingResult === 'excluded' && specialChildren.has(pal.id)) throw new Error(`${pal.id}: excluded but produced by a special combination`)
  }
  return data
}

function compareTuple(left, right) {
  for (let index = 0; index < left.length; index += 1) {
    const result = typeof left[index] === 'number' ? left[index] - right[index] : left[index] < right[index] ? -1 : left[index] > right[index] ? 1 : 0
    if (result !== 0) return result
  }
  return 0
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const target = resolve(process.cwd(), process.argv[2] ?? 'src/generated/breeding-data.json')
  const data = JSON.parse(await readFile(target, 'utf8'))
  await validateBreedingData(data)
  console.log(`Valid breeding data: ${target}; pals=${data.pals.length}; specialCombinations=${data.specialCombinations.length}`)
}
