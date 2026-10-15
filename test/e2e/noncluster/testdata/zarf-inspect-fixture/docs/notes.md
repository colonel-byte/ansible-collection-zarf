# Notes

A second documentation key, so that the extraction tests cover a package carrying more than one and
a file that is not YAML. `zarf_package_documentation` returns this as text in `documents` and does
not put it in `parsed`, because parsing a Markdown file as YAML succeeds and returns the whole
document as one string -- which looks like a parse and is not.
