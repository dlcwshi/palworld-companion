export type NormalBreedingResult = 'eligible' | 'specialOnly' | 'excluded'
export type BreedingGender = 'any' | 'male' | 'female'

export interface BreedingMetadata {
    schemaVersion: 1
    gameVersion: string
    steamBuildId: string
    generatedAt: string
    generatorVersion: string
    sourceHash: string
}

export interface BreedingPal {
    id: string
    dexNo: number
    dexSuffix?: string
    nameZh: string
    nameEn: string
    breedingPower: number
    breedingOrder: number
    normalBreedingResult: NormalBreedingResult
}

export interface SpecialBreedingCombination {
    parentA: string
    parentAGender: BreedingGender
    parentB: string
    parentBGender: BreedingGender
    child: string
}

export interface BreedingData {
  metadata: BreedingMetadata
  pals: BreedingPal[]
  specialCombinations: SpecialBreedingCombination[]
}
