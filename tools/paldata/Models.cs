using System.Text.Json.Serialization;

namespace PalworldCompanion.PalData;

public sealed record BreedingData(BreedingMetadata Metadata, IReadOnlyList<BreedingPal> Pals,
    IReadOnlyList<SpecialBreedingCombination> SpecialCombinations);
public sealed record BreedingMetadata(int SchemaVersion, string GameVersion, string SteamBuildId,
    DateTimeOffset GeneratedAt, string GeneratorVersion, string SourceHash);
public sealed record BreedingPal(string Id, int DexNo,
    [property: JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] string? DexSuffix,
    string NameZh, string NameEn, int BreedingPower, int BreedingOrder, NormalBreedingResult NormalBreedingResult);
[JsonConverter(typeof(JsonStringEnumConverter<NormalBreedingResult>))]
public enum NormalBreedingResult { Eligible, SpecialOnly, Excluded }
public sealed record SpecialBreedingCombination(string ParentA, BreedingGender ParentAGender,
    string ParentB, BreedingGender ParentBGender, string Child);
[JsonConverter(typeof(JsonStringEnumConverter<BreedingGender>))]
public enum BreedingGender { Any, Male, Female }
public sealed record RawPal(string Id, string TribeId, int DexNo, string? DexSuffix, string NameTextId,
    string NameZh, string NameEn, int CombiRank, int CombiDuplicatePriority, bool IgnoreCombi);
public sealed record RawSpecialCombination(string ParentA, RawGender ParentAGender, string ParentB,
    RawGender ParentBGender, string Child);
public enum RawGender { Any, Male, Female }
public sealed record RawExtraction(string GameVersion, string SteamBuildId, IReadOnlyList<RawPal> Pals,
    IReadOnlyList<RawSpecialCombination> SpecialCombinations, IReadOnlyList<SourceFingerprint> Sources);
public sealed record SourceFingerprint(string LogicalPath, string Sha256);
public interface IPalworldDataSource { Task<RawExtraction> ExtractAsync(CancellationToken cancellationToken = default); }
public sealed class InputException(string message) : Exception(message);
public sealed class ExtractionException(string message, Exception? inner = null) : Exception(message, inner);
public sealed class DataValidationException(string message) : Exception(message);
