# Third-party logos

The logos in this directory are the official marks of the projects they name, used unmodified to
label the settings of the data store each project provides. They are not GitLab marks, and their
presence implies no affiliation with or endorsement by either project.

| File             | Source                                                                                                                                      | Terms                                                                                                                                                                                             |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `postgresql.svg` | [PostgreSQL logo](https://wiki.postgresql.org/wiki/Logo) (`PostgreSQL_logo.3colors.svg`)                                                    | The Elephant Logo (Slonik) is a registered trademark of the PostgreSQL Community Association of Canada, used under the [trademark policy](https://www.postgresql.org/about/policies/trademarks/). |
| `bucket.svg`     | [Bootstrap Icons](https://icons.getbootstrap.com/icons/bucket-fill/) (`bucket-fill`)                                                        | MIT. Not a logo: object storage is a concept rather than a product, so the form labels it with a plain glyph.                                                                                     |
| `valkey.svg`     | [valkey-io.github.io](https://github.com/valkey-io/valkey-io.github.io/blob/main/static/img/Valkey-logo.svg) (`static/img/Valkey-logo.svg`) | Copyright Linux Foundation or its affiliates, from the BSD-3-Clause licensed Valkey website repository.                                                                                           |

The PostgreSQL logo carries a white outline of its own, so it renders unmodified on both themes.
The Valkey mark is one solid dark shape with no light variant published, so the dark theme renders
it as a white silhouette (`.legend-icon--mono` in
[GitLabFormView.vue](../../views/GitLabFormView.vue)); its cutouts are holes in the path, so the
shape survives. A single-color rendition is the usual allowance in a trademark policy. Do not
flatten the PostgreSQL logo the same way, which would throw away the outlines that draw the
elephant, and do not recolor either mark in any other way.

`bucket.svg` is painted through a CSS mask (`.legend-icon--glyph`) rather than shown as an image, so
it takes the text color and needs no variant per theme. It ships `fill="currentColor"`, which an
`<img>` element ignores.

Replace a file only with the current official artwork from the source above, and keep this table in
step.
