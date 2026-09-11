# Use the preview directory as the serving boundary

Static mockups commonly reference nearby stylesheets, scripts, and images, so selecting a preview page makes its preview directory the serving boundary: ordinary files anywhere beneath it can be fetched by a known URL, whether or not the page references them.
Directory listings and dotfiles or dot-directories within that boundary are hidden, and request paths or symlinks must not expose files outside it.
This contract supports local assets without analyzing page dependencies; users who want to share fewer files place the preview page and its assets in a dedicated preview directory.
