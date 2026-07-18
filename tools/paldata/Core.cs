using System.Security.Cryptography;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;

namespace PalworldCompanion.PalData;

public sealed record CliOptions(string? GameDirectory, string OutputPath, string? UsmapPath, string? OodlePath,
    DateTimeOffset? GeneratedAt, bool Pretty, bool VerifyOnly, bool Verbose)
{
    public const string HelpText = """
Palworld Companion local breeding-data generator

Usage:
  dotnet run --project tools/paldata -- --game-dir <path> --out <file> [options]
  dotnet run --project tools/paldata -- --verify-only --out <file>

Options:
  --game-dir <path>       Palworld Windows client root.
  --out <path>            Target breeding-data.json.
  --usmap <path>          Optional local mapping file; never downloaded automatically.
  --oodle <path>         Local oo2core_9_win64.dll or compatible Oodle library; never downloaded automatically.
  --generated-at <value>  Fixed ISO-8601 UTC timestamp for reproducible output.
  --pretty                Write indented JSON.
  --verify-only           Validate an existing output without reading game files.
  --verbose               Print extraction diagnostics.
  --help                  Show this help.
""";

    public static (CliOptions? Options, string? Error, bool Help) Parse(IReadOnlyList<string> args)
    {
        if (args.Count == 0) return (null, "Missing required arguments.", false);
        string? gameDir = null, output = null, usmap = null, oodle = null;
        DateTimeOffset? generatedAt = null;
        var pretty = false; var verifyOnly = false; var verbose = false;
        for (var index = 0; index < args.Count; index++)
        {
            switch (args[index])
            {
                case "--help": case "-h": return (null, null, true);
                case "--pretty": pretty = true; break;
                case "--verify-only": verifyOnly = true; break;
                case "--verbose": verbose = true; break;
                case "--game-dir": gameDir = ReadValue(args, ref index, "--game-dir", out var ge); if (ge is not null) return (null, ge, false); break;
                case "--out": output = ReadValue(args, ref index, "--out", out var oe); if (oe is not null) return (null, oe, false); break;
                case "--usmap": usmap = ReadValue(args, ref index, "--usmap", out var me); if (me is not null) return (null, me, false); break;
                case "--oodle": oodle = ReadValue(args, ref index, "--oodle", out var de); if (de is not null) return (null, de, false); break;
                case "--generated-at":
                    var value = ReadValue(args, ref index, "--generated-at", out var te);
                    if (te is not null) return (null, te, false);
                    if (value is null || !value.EndsWith('Z') || !DateTimeOffset.TryParse(value,
                            System.Globalization.CultureInfo.InvariantCulture,
                            System.Globalization.DateTimeStyles.AssumeUniversal | System.Globalization.DateTimeStyles.AdjustToUniversal,
                            out var timestamp)) return (null, "--generated-at must be an ISO-8601 UTC timestamp ending in Z.", false);
                    generatedAt = timestamp; break;
                default: return (null, $"Unknown argument: {args[index]}", false);
            }
        }
        if (string.IsNullOrWhiteSpace(output)) return (null, "Missing required --out <path>.", false);
        if (!verifyOnly && string.IsNullOrWhiteSpace(gameDir)) return (null, "Missing required --game-dir <path>.", false);
        return (new(gameDir, output, usmap, oodle, generatedAt, pretty, verifyOnly, verbose), null, false);
    }

    private static string? ReadValue(IReadOnlyList<string> args, ref int index, string option, out string? error)
    {
        if (++index >= args.Count || args[index].StartsWith("--", StringComparison.Ordinal))
        { error = $"{option} requires a value."; return null; }
        error = null; return args[index];
    }
}

public static class PalDataApplication
{
    public const string GeneratorVersion = "0.1.0";
    public static async Task<int> RunAsync(string[] args, TextWriter output, TextWriter error,
        Func<CliOptions, IPalworldDataSource>? sourceFactory = null)
    {
        var parsed = CliOptions.Parse(args);
        if (parsed.Help) { await output.WriteLineAsync(CliOptions.HelpText); return 0; }
        if (parsed.Error is not null) { await error.WriteLineAsync(parsed.Error); return 2; }
        var options = parsed.Options!;
        try
        {
            ValidateOutputPath(options.OutputPath);
            if (options.VerifyOnly)
            {
                var existing = await StableJson.ReadAsync(options.OutputPath);
                BreedingDataValidator.Validate(existing);
                await output.WriteLineAsync($"Valid breeding data: pals={existing.Pals.Count}, specialCombinations={existing.SpecialCombinations.Count}");
                return 0;
            }
            if (options.UsmapPath is not null && !File.Exists(options.UsmapPath))
                throw new InputException($"Mapping file does not exist: {options.UsmapPath}");
            if (options.OodlePath is not null && !File.Exists(options.OodlePath))
                throw new InputException($"Oodle library does not exist: {options.OodlePath}");
            GameInstallation.Validate(options.GameDirectory!);
            sourceFactory ??= static cli => new Cue4ParseDataSource(cli.GameDirectory!, cli.UsmapPath, cli.OodlePath, cli.Verbose);
            var data = await BreedingDataGenerator.GenerateAsync(sourceFactory(options),
                options.GeneratedAt ?? DateTimeOffset.UtcNow, GeneratorVersion);
            await StableJson.WriteAtomicallyAsync(options.OutputPath, data, options.Pretty);
            await output.WriteLineAsync($"Generated {options.OutputPath}");
            await output.WriteLineAsync($"gameVersion={data.Metadata.GameVersion}");
            await output.WriteLineAsync($"steamBuildId={data.Metadata.SteamBuildId}");
            await output.WriteLineAsync($"pals={data.Pals.Count}");
            await output.WriteLineAsync($"specialCombinations={data.SpecialCombinations.Count}");
            await output.WriteLineAsync($"sourceHash={data.Metadata.SourceHash}");
            return 0;
        }
        catch (InputException ex) { await error.WriteLineAsync(ex.Message); return 3; }
        catch (ExtractionException ex) { await error.WriteLineAsync(ex.Message); return 4; }
        catch (DataValidationException ex) { await error.WriteLineAsync(ex.Message); return 5; }
        catch (Exception ex) { await error.WriteLineAsync($"Unexpected generator failure: {ex.Message}"); return 10; }
    }

    private static void ValidateOutputPath(string outputPath)
    {
        if (string.IsNullOrWhiteSpace(Path.GetFileName(outputPath)) ||
            !string.Equals(Path.GetExtension(outputPath), ".json", StringComparison.OrdinalIgnoreCase))
            throw new InputException($"Output path must name a .json file: {outputPath}");
        var parent = Path.GetDirectoryName(Path.GetFullPath(outputPath));
        if (parent is null || !Directory.Exists(parent)) throw new InputException($"Output directory does not exist: {parent ?? outputPath}");
    }
}

public static class GameInstallation
{
    public static void Validate(string gameDirectory)
    {
        if (!Directory.Exists(gameDirectory)) throw new InputException($"Palworld game directory does not exist: {gameDirectory}");
        foreach (var relative in new[] { "Palworld.exe", "Pal", "Engine", Path.Combine("Pal", "Content", "Paks", "Pal-Windows.pak") })
        {
            var path = Path.Combine(gameDirectory, relative);
            if (!File.Exists(path) && !Directory.Exists(path)) throw new InputException($"Palworld game directory is missing required item: {relative}");
        }
    }
    public static string FindManifest(string gameDirectory)
    {
        var current = new DirectoryInfo(Path.GetFullPath(gameDirectory));
        while (current is not null)
        {
            var candidate = Path.Combine(current.FullName, "appmanifest_1623730.acf");
            if (File.Exists(candidate)) return candidate;
            current = current.Parent;
        }
        throw new ExtractionException("Could not associate --game-dir with Steam appmanifest_1623730.acf; Steam Web API access is disabled.");
    }
    public static string ReadBuildId(string manifestPath)
    {
        var match = Regex.Match(File.ReadAllText(manifestPath), "\\\"buildid\\\"\\s+\\\"([0-9]+)\\\"", RegexOptions.CultureInvariant);
        if (!match.Success) throw new ExtractionException($"Steam manifest has no buildid: {manifestPath}");
        return match.Groups[1].Value;
    }
}

public static class SourceHasher
{
    public static string HashCanonical<T>(T value) => Convert.ToHexString(SHA256.HashData(
        JsonSerializer.SerializeToUtf8Bytes(value, StableJson.CompactOptions))).ToLowerInvariant();
    public static string HashFile(string path) { using var stream = File.OpenRead(path); return Convert.ToHexString(SHA256.HashData(stream)).ToLowerInvariant(); }
    public static string HashBytes(ReadOnlySpan<byte> bytes) => Convert.ToHexString(SHA256.HashData(bytes)).ToLowerInvariant();
}

public static class BreedingDataGenerator
{
    public static async Task<BreedingData> GenerateAsync(IPalworldDataSource source, DateTimeOffset generatedAt,
        string generatorVersion, CancellationToken cancellationToken = default)
    {
        var raw = await source.ExtractAsync(cancellationToken);
        var childIds = raw.SpecialCombinations.Select(item => item.Child).ToHashSet(StringComparer.Ordinal);
        var pals = raw.Pals.Select(item => new BreedingPal(item.Id, item.DexNo,
                string.IsNullOrWhiteSpace(item.DexSuffix) ? null : item.DexSuffix, CleanName(item.NameZh), CleanName(item.NameEn),
                item.CombiRank, item.CombiDuplicatePriority, !item.IgnoreCombi ? NormalBreedingResult.Eligible :
                    childIds.Contains(item.Id) ? NormalBreedingResult.SpecialOnly : NormalBreedingResult.Excluded))
            .OrderBy(item => item.DexNo).ThenBy(item => item.DexSuffix ?? string.Empty, StringComparer.Ordinal)
            .ThenBy(item => item.Id, StringComparer.Ordinal).ToArray();
        var combinations = raw.SpecialCombinations.Select(Normalize)
            .OrderBy(item => item.ParentA, StringComparer.Ordinal).ThenBy(item => item.ParentB, StringComparer.Ordinal)
            .ThenBy(item => item.ParentAGender).ThenBy(item => item.ParentBGender)
            .ThenBy(item => item.Child, StringComparer.Ordinal).ToArray();
        var canonicalSource = new { raw.GameVersion, raw.SteamBuildId,
            Pals = raw.Pals.OrderBy(item => item.Id, StringComparer.Ordinal),
            SpecialCombinations = raw.SpecialCombinations.OrderBy(item => item.ParentA, StringComparer.Ordinal).ThenBy(item => item.ParentB, StringComparer.Ordinal),
            Sources = raw.Sources.OrderBy(item => item.LogicalPath, StringComparer.Ordinal) };
        var data = new BreedingData(new(1, raw.GameVersion, raw.SteamBuildId, generatedAt.ToUniversalTime(),
            generatorVersion, SourceHasher.HashCanonical(canonicalSource)), pals, combinations);
        BreedingDataValidator.Validate(data);
        return data;
    }
    private static string CleanName(string value) => new string(value.Where(character => !char.IsControl(character) || character is '\t' or '\n').ToArray()).Trim();
    private static SpecialBreedingCombination Normalize(RawSpecialCombination item)
    {
        var ag = ToGender(item.ParentAGender); var bg = ToGender(item.ParentBGender);
        return string.CompareOrdinal(item.ParentA, item.ParentB) <= 0 ? new(item.ParentA, ag, item.ParentB, bg, item.Child) :
            new(item.ParentB, bg, item.ParentA, ag, item.Child);
    }
    private static BreedingGender ToGender(RawGender value) => value switch
    { RawGender.Any => BreedingGender.Any, RawGender.Male => BreedingGender.Male, RawGender.Female => BreedingGender.Female,
        _ => throw new DataValidationException($"Unsupported breeding gender: {value}") };
}

public static class BreedingDataValidator
{
    private static readonly Regex IdPattern = new("^[A-Za-z0-9_]+$", RegexOptions.CultureInvariant);
    private static readonly Regex HashPattern = new("^[0-9a-f]{64}$", RegexOptions.CultureInvariant);
    public static void Validate(BreedingData data)
    {
        var errors = new List<string>();
        if (data.Metadata.SchemaVersion != 1) errors.Add("metadata.schemaVersion must equal 1");
        if (string.IsNullOrWhiteSpace(data.Metadata.GameVersion)) errors.Add("metadata.gameVersion is required");
        if (!Regex.IsMatch(data.Metadata.SteamBuildId, "^[0-9]+$")) errors.Add("metadata.steamBuildId must contain digits only");
        if (data.Metadata.GeneratedAt.Offset != TimeSpan.Zero) errors.Add("metadata.generatedAt must be UTC");
        if (string.IsNullOrWhiteSpace(data.Metadata.GeneratorVersion)) errors.Add("metadata.generatorVersion is required");
        if (!HashPattern.IsMatch(data.Metadata.SourceHash)) errors.Add("metadata.sourceHash must be lowercase SHA-256");
        if (data.Pals.Count == 0) errors.Add("pals must contain at least one record");
        var ids = new HashSet<string>(StringComparer.Ordinal); BreedingPal? previous = null;
        foreach (var pal in data.Pals)
        {
            if (!IdPattern.IsMatch(pal.Id)) errors.Add($"invalid pal id: {pal.Id}");
            if (!ids.Add(pal.Id)) errors.Add($"duplicate pal id: {pal.Id}");
            if (pal.DexNo <= 0) errors.Add($"{pal.Id}: dexNo must be positive");
            if (pal.DexSuffix is not null && !Regex.IsMatch(pal.DexSuffix, "^[A-Za-z0-9]+$")) errors.Add($"{pal.Id}: invalid dexSuffix");
            if (string.IsNullOrWhiteSpace(pal.NameZh) || string.IsNullOrWhiteSpace(pal.NameEn)) errors.Add($"{pal.Id}: localized names are required");
            if (pal.BreedingPower < 0 || pal.BreedingPower > 10000) errors.Add($"{pal.Id}: breedingPower out of range");
            if (pal.BreedingOrder < 0 || pal.BreedingOrder > 10000) errors.Add($"{pal.Id}: breedingOrder out of range");
            if (previous is not null && Compare(previous, pal) >= 0) errors.Add($"pals are not in stable order at {pal.Id}");
            previous = pal;
        }
        var pairs = new Dictionary<string, string>(StringComparer.Ordinal); SpecialBreedingCombination? prior = null;
        var specialChildren = new HashSet<string>(StringComparer.Ordinal);
        foreach (var combo in data.SpecialCombinations)
        {
            if (!ids.Contains(combo.ParentA) || !ids.Contains(combo.ParentB) || !ids.Contains(combo.Child)) errors.Add($"special combination references an unknown id: {combo.ParentA}+{combo.ParentB}->{combo.Child}");
            if (string.CompareOrdinal(combo.ParentA, combo.ParentB) > 0) errors.Add($"special combination parents are not normalized: {combo.ParentA}+{combo.ParentB}");
            var key = combo.ParentA + "\0" + combo.ParentAGender + "\0" + combo.ParentB + "\0" + combo.ParentBGender;
            if (pairs.TryGetValue(key, out var child)) errors.Add(child == combo.Child ? $"duplicate special combination: {combo.ParentA}+{combo.ParentB}" : $"conflicting special combination: {combo.ParentA}+{combo.ParentB}");
            else pairs[key] = combo.Child;
            specialChildren.Add(combo.Child);
            if (prior is not null && Compare(prior, combo) >= 0) errors.Add("specialCombinations are not in stable order");
            prior = combo;
        }
        foreach (var pal in data.Pals)
        {
            var special = specialChildren.Contains(pal.Id);
            if (pal.NormalBreedingResult == NormalBreedingResult.SpecialOnly && !special) errors.Add($"{pal.Id}: specialOnly without a special combination");
            if (pal.NormalBreedingResult == NormalBreedingResult.Excluded && special) errors.Add($"{pal.Id}: excluded but produced by a special combination");
        }
        if (errors.Count > 0) throw new DataValidationException("Breeding data validation failed:\n- " + string.Join("\n- ", errors));
    }
    private static int Compare(BreedingPal a, BreedingPal b) { var v = a.DexNo.CompareTo(b.DexNo); if (v != 0) return v;
        v = string.Compare(a.DexSuffix ?? "", b.DexSuffix ?? "", StringComparison.Ordinal); return v != 0 ? v : string.Compare(a.Id, b.Id, StringComparison.Ordinal); }
    private static int Compare(SpecialBreedingCombination a, SpecialBreedingCombination b) { var v = string.Compare(a.ParentA, b.ParentA, StringComparison.Ordinal); if (v != 0) return v;
        v = string.Compare(a.ParentB, b.ParentB, StringComparison.Ordinal); if (v != 0) return v;
        v = a.ParentAGender.CompareTo(b.ParentAGender); if (v != 0) return v;
        v = a.ParentBGender.CompareTo(b.ParentBGender); return v != 0 ? v : string.Compare(a.Child, b.Child, StringComparison.Ordinal); }
}

public static class StableJson
{
    public static readonly JsonSerializerOptions CompactOptions = Create(false);
    private static readonly JsonSerializerOptions PrettyOptions = Create(true);
    public static async Task<BreedingData> ReadAsync(string path)
    {
        if (!File.Exists(path)) throw new InputException($"Breeding data file does not exist: {path}");
        await using var stream = File.OpenRead(path);
        return await JsonSerializer.DeserializeAsync<BreedingData>(stream, CompactOptions) ?? throw new DataValidationException("Breeding data JSON is empty.");
    }
    public static byte[] Serialize(BreedingData data, bool pretty) => new UTF8Encoding(false).GetBytes(
        JsonSerializer.Serialize(data, pretty ? PrettyOptions : CompactOptions).Replace("\r\n", "\n", StringComparison.Ordinal) + "\n");
    public static async Task WriteAtomicallyAsync(string path, BreedingData data, bool pretty)
    {
        BreedingDataValidator.Validate(data); var full = Path.GetFullPath(path); var temp = full + ".tmp-" + Guid.NewGuid().ToString("N");
        try { await File.WriteAllBytesAsync(temp, Serialize(data, pretty)); BreedingDataValidator.Validate(await ReadAsync(temp)); File.Move(temp, full, true); }
        finally { if (File.Exists(temp)) File.Delete(temp); }
    }
    private static JsonSerializerOptions Create(bool indented)
    {
        var options = new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.CamelCase, WriteIndented = indented,
            Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };
        options.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.CamelCase)); return options;
    }
}
