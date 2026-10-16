# Notes

A second documentation key, so that the fixture carries more than one and so that one of them is
not YAML. The distinction matters to anything that reads the layer rather than the mapping:
parsing a Markdown file as YAML succeeds and hands back the whole document as one string, which
looks like a parse and is not.
