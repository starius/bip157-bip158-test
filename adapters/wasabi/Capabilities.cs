namespace WasabiAdapter;

/// <summary>
/// Environment support reported by the Wasabi adapter.
/// </summary>
internal static class AdapterCapabilities
{
    /// <summary>
    /// Returns the environments this adapter currently claims as active.
    /// </summary>
    public static CapabilitiesResponse ClearIPv4Only() =>
        new(new[]
        {
            new EnvironmentCapability("ipv4", true),
            new EnvironmentCapability("ipv6", false, "adapter has not been validated with IPv6 peer identities"),
            new EnvironmentCapability("tor-v3", false, "adapter does not configure Tor proxying"),
            new EnvironmentCapability("i2p", false, "adapter does not configure I2P proxying"),
            new EnvironmentCapability("cjdns", false, "adapter has not been validated with cjdns")
        });
}
