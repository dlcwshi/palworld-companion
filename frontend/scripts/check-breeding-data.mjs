import assert from 'node:assert/strict'
import { validateBreedingData } from './verify-breeding-data.mjs'

const valid = {
  metadata: { schemaVersion: 1, gameVersion: 'test', steamBuildId: '1', generatedAt: '2026-07-18T00:00:00Z', generatorVersion: 'test', sourceHash: 'a'.repeat(64) },
  pals: [
    { id: 'A', dexNo: 1, nameZh: '甲', nameEn: 'A', breedingPower: 1, breedingOrder: 1, normalBreedingResult: 'eligible' },
    { id: 'B', dexNo: 2, nameZh: '乙', nameEn: 'B', breedingPower: 2, breedingOrder: 2, normalBreedingResult: 'specialOnly' }
  ],
  specialCombinations: [{ parentA: 'A', parentAGender: 'female', parentB: 'B', parentBGender: 'male', child: 'B' }]
}
await validateBreedingData(valid)
await assert.rejects(() => validateBreedingData({ ...valid, pals: [valid.pals[0], valid.pals[0]] }), /duplicate pal id/)
await assert.rejects(() => validateBreedingData({ ...valid, specialCombinations: [{ ...valid.specialCombinations[0], child: 'Missing' }] }), /unknown id/)
console.log('Breeding data schema and semantic contract checks passed.')
