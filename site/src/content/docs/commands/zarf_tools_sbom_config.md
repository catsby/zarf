---
title: zarf tools sbom config
description: Zarf CLI command reference for <code>zarf tools sbom config</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf tools sbom config

show the syft configuration

```
zarf tools sbom config [flags]
```

### Options

```
  -h, --help   help for config
      --load   load and validate the syft configuration
```

### Options inherited from parent commands

```
  -c, --config string              syft configuration file
      --insecure-skip-tls-verify   Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --plain-http                 Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -q, --quiet                      suppress all logging output
  -v, --verbose count              increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools sbom](/commands/zarf_tools_sbom/)	 - Generates a Software Bill of Materials (SBOM) for the given package
* [zarf tools sbom config locations](/commands/zarf_tools_sbom_config_locations/)	 - shows all locations and the order in which syft will look for a configuration file
