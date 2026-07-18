using System.Text;
using PalworldCompanion.PalData;
using Xunit;

namespace PalworldCompanion.PalData.Tests;

public sealed class PalDataTests
{
    private static readonly DateTimeOffset FixedTime = DateTimeOffset.Parse("2026-07-18T00:00:00Z");

    [Fact]
    public void CliRequiresArgumentsAndSupportsOfflineResources()
    {
        Assert.Equal("Missing required arguments.", CliOptions.Parse([]).Error);
        var parsed = CliOptions.Parse(["--game-dir", "game", "--out", "data.json", "--usmap", "map.usmap",
            "--oodle", "oo2core_9_win64.dll", "--generated-at", "2026-07-18T00:00:00Z", "--pretty"]);
        Assert.NotNull(parsed.Options);
        Assert.Equal("map.usmap", parsed.Options!.UsmapPath);
        Assert.Equal("oo2core_9_win64.dll", parsed.Options.OodlePath);
        Assert.True(parsed.Options.Pretty);
    }

    [Fact]
    public async Task CliHelpAndInputFailuresReturnDocumentedExitCodes()
    {
        var output = new StringWriter();
        var error = new StringWriter();
        Assert.Equal(0, await PalDataApplication.RunAsync(["--help"], output, error));
        Assert.Contains("--generated-at", output.ToString());

        var target = Path.Combine(Environment.CurrentDirectory, "paldata-test-output.json");
        Assert.Equal(3, await PalDataApplication.RunAsync(["--game-dir", "missing-game", "--out", target], output, error));
        Assert.Contains("game directory does not exist", error.ToString());

        error.GetStringBuilder().Clear();
        Assert.Equal(3, await PalDataApplication.RunAsync(["--game-dir", "missing-game", "--out", "invalid.txt"], output, error));
        Assert.Contains("must name a .json file", error.ToString());

        error.GetStringBuilder().Clear();
        Assert.Equal(3, await PalDataApplication.RunAsync(["--game-dir", "missing-game", "--out", target, "--usmap", "missing.usmap"], output, error));
        Assert.Contains("Mapping file does not exist", error.ToString());
    }

    [Fact]
    public async Task GenerationIsStableAndNormalizesParents()
    {
        var source = new FakeSource();
        var first = await BreedingDataGenerator.GenerateAsync(source, FixedTime, "test");
        var second = await BreedingDataGenerator.GenerateAsync(source, FixedTime, "test");
        Assert.Equal(StableJson.Serialize(first, false), StableJson.Serialize(second, false));
        Assert.Equal("A", first.SpecialCombinations[0].ParentA);
        Assert.Equal("B", first.SpecialCombinations[0].ParentB);
        var bytes = StableJson.Serialize(first, true);
        Assert.False(bytes.AsSpan().StartsWith(Encoding.UTF8.GetPreamble()));
        Assert.DoesNotContain((byte)'\r', bytes);
        Assert.Equal((byte)'\n', bytes[^1]);
        Assert.Equal(NormalBreedingResult.SpecialOnly, first.Pals.Single(item => item.Id == "C").NormalBreedingResult);
    }

    [Fact]
    public void ValidatorRejectsDuplicateIdsAndBrokenReferences()
    {
        var valid = ValidData();
        var data = valid with
        {
            Pals = [valid.Pals[0], valid.Pals[0]],
            SpecialCombinations = [new("A", BreedingGender.Any, "missing", BreedingGender.Any, "A")]
        };
        var exception = Assert.Throws<DataValidationException>(() => BreedingDataValidator.Validate(data));
        Assert.Contains("duplicate pal id", exception.Message);
        Assert.Contains("unknown id", exception.Message);
    }

    [Theory]
    [InlineData("", 10, 10)]
    [InlineData("English", -1, 10)]
    [InlineData("English", 10, 10001)]
    public void ValidatorRejectsNamesAndRanges(string englishName, int power, int order)
    {
        var valid = ValidData();
        var bad = valid with { Pals = [valid.Pals[0] with { NameEn = englishName, BreedingPower = power, BreedingOrder = order }] };
        Assert.Throws<DataValidationException>(() => BreedingDataValidator.Validate(bad));
    }

    [Fact]
    public void CanonicalHashIsIndependentOfDictionaryInsertionOrder()
    {
        var first = new SortedDictionary<string, int>(StringComparer.Ordinal) { ["a"] = 1, ["b"] = 2 };
        var second = new SortedDictionary<string, int>(StringComparer.Ordinal) { ["b"] = 2, ["a"] = 1 };
        Assert.Equal(SourceHasher.HashCanonical(first), SourceHasher.HashCanonical(second));
        Assert.NotEqual(SourceHasher.HashCanonical(first), SourceHasher.HashCanonical(new SortedDictionary<string, int>(StringComparer.Ordinal) { ["a"] = 2, ["b"] = 2 }));
    }

    [Fact]
    public void ValidatorRejectsDuplicateConflictingAndUnnormalizedSpecialRules()
    {
        var valid = ValidData();
        var rule = new SpecialBreedingCombination("A", BreedingGender.Any, "B", BreedingGender.Any, "A");
        Assert.Throws<DataValidationException>(() => BreedingDataValidator.Validate(valid with { SpecialCombinations = [rule, rule] }));
        Assert.Throws<DataValidationException>(() => BreedingDataValidator.Validate(valid with
        {
            SpecialCombinations = [rule, rule with { Child = "B" }]
        }));
        Assert.Throws<DataValidationException>(() => BreedingDataValidator.Validate(valid with
        {
            SpecialCombinations = [rule with { ParentA = "B", ParentB = "A" }]
        }));
    }

    [Fact]
    public void ValidatorAllowsGenderSpecificRulesForTheSameParents()
    {
        var valid = ValidData();
        var data = valid with
        {
            SpecialCombinations =
            [
                new("A", BreedingGender.Any, "A", BreedingGender.Male, "A"),
                new("A", BreedingGender.Male, "A", BreedingGender.Female, "A")
            ]
        };
        BreedingDataValidator.Validate(data);
    }

    private static BreedingData ValidData() => new(new(1, "0.6.6.74520", "24181527", FixedTime, "test",
        new string('a', 64)),
        [new("A", 1, null, "甲", "English", 10, 10, NormalBreedingResult.Eligible),
         new("B", 2, null, "乙", "Bee", 20, 20, NormalBreedingResult.Eligible)], []);

    private sealed class FakeSource : IPalworldDataSource
    {
        public Task<RawExtraction> ExtractAsync(CancellationToken cancellationToken = default) => Task.FromResult(new RawExtraction(
            "0.6.6.74520", "24181527",
            [new("B", "B", 2, null, "B", "乙", "Bee", 20, 2, false),
             new("A", "A", 1, null, "A", "甲", "Aye", 10, 1, false),
             new("C", "C", 3, null, "C", "丙", "See", 30, 3, true)],
            [new("B", RawGender.Male, "A", RawGender.Female, "C")],
            [new("source-b", new string('b', 64)), new("source-a", new string('a', 64))]));
    }
}
