using NBitcoin;
using WasabiAdapter;

var listen = ParseListen(args);
var state = new AdapterState();
var builder = WebApplication.CreateBuilder(args);
builder.Logging.ClearProviders();
builder.WebHost.UseUrls($"http://{listen}");
var app = builder.Build();

app.MapPost("/health", () =>
{
    var status = state.Client is not null ? "started" : state.Config is not null ? "configured" : "idle";
    return Results.Json(new HealthResponse(true, status));
});

app.MapPost("/configure", async (ConfigureRequest request) =>
{
    if (!string.Equals(request.Network, "regtest", StringComparison.OrdinalIgnoreCase))
    {
        return Results.BadRequest("only regtest is supported");
    }

    foreach (var peer in request.Peers)
    {
        try
        {
            _ = WasabiP2pClient.ParseEndpoint(peer.Address);
        }
        catch (Exception ex)
        {
            return Results.BadRequest($"bad peer address {peer.Address}: {ex.Message}");
        }
    }

    var dataDir = string.IsNullOrWhiteSpace(request.DataDir)
        ? Path.Combine(Path.GetTempPath(), $"wasabi-adapter-{Guid.NewGuid():N}")
        : request.DataDir;
    await state.ConfigureAsync(request with { DataDir = dataDir }).ConfigureAwait(false);
    return Results.Json(new { ok = true });
});

app.MapPost("/start", async (CancellationToken cancellationToken) =>
{
    if (state.Config is not { } config)
    {
        return Results.Conflict("configure before start");
    }

    if (state.Client is not null)
    {
        return Results.Json(new { ok = true });
    }

    var client = await WasabiP2pClient.CreateAsync(state, config, cancellationToken)
        .ConfigureAwait(false);
    state.SetClient(client);
    return Results.Json(new { ok = true });
});

app.MapPost("/stop", async () =>
{
    await state.StopAsync().ConfigureAwait(false);
    return Results.Json(new { ok = true });
});

app.MapPost("/watch-script", (WatchScriptRequest request) =>
{
    try
    {
        var script = new Script(Convert.FromHexString(request.ScriptPubKeyHex));
        state.AddWatch(request, script);
        return Results.Json(new { ok = true });
    }
    catch (Exception ex)
    {
        return Results.BadRequest($"invalid script: {ex.Message}");
    }
});

app.MapPost("/best-block", () =>
{
    if (state.Client is not { } client)
    {
        return Results.Conflict("adapter not started");
    }
    return Results.Json(client.BestBlock());
});

app.MapPost("/block-hash", (BlockRef request) =>
{
    if (state.Client is not { } client)
    {
        return Results.Conflict("adapter not started");
    }

    var block = client.BlockHash(request.Height);
    return block is null ? Results.NotFound("height not known") : Results.Json(block);
});

app.MapPost("/matches", (GetMatchesRequest request) =>
    Results.Json(new GetMatchesResponse(state.Matches(request))));

app.MapPost("/list-peers", () =>
{
    if (state.Client is not { } client)
    {
        return Results.Json(new ListPeersResponse([]));
    }
    return Results.Json(new ListPeersResponse(client.ListPeers()));
});

await app.StartAsync().ConfigureAwait(false);
var address = app.Urls.First();
Console.WriteLine($"listening={address}");
await app.WaitForShutdownAsync().ConfigureAwait(false);

static string ParseListen(string[] args)
{
    for (var i = 0; i < args.Length - 1; i++)
    {
        if (args[i] == "--listen")
        {
            return args[i + 1];
        }
    }
    return "127.0.0.1:0";
}
