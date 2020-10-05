# Usage:

Compile the extension with:

```sh
$ make
```

And in the sqlite prompt, load the extension with:

```
sqlite> .load ./helpers
```

Then you'll be able to use the following functions:
- lm_resolve_domain_mapping(domain: string)
