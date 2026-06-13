# Security policy

## Supported versions

stdocs is pre-1.0. Security fixes land on `main` and ship in the next
release; please run a recent version.

## Reporting a vulnerability

Please report security issues privately, not through public issues or
pull requests.

Use GitHub's private vulnerability reporting — open the **Security**
tab on this repository and choose **Report a vulnerability**. That
opens a private advisory visible only to you and the maintainer.

You can expect an initial acknowledgement within a few days. Once a
fix is ready, it ships in a tagged release and the advisory is
published with credit to the reporter unless you prefer otherwise.

stdocs has no runtime dependencies beyond the Go standard library, so
the surface is small: the most likely areas are the generated
document and the served docs endpoints. Reports about the bundled
documentation UIs (Scalar, Swagger UI, Redoc, Stoplight Elements)
are best filed upstream, but let us know so the pinned version can be
bumped.
