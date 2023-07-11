Package `simplefs` provides an interface for working with a file
system as well as default implementations for doing this fully
in-memory and through the `os` package.

This package was originally created before the `io/fs` package
existed and used in a project where we needed the same kind of
abstraction. Features may therefore overlap with `io/fs` and will
probably also be less optimal / optimized to work with.