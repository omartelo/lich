// Keys: update.<module>.<what>, the module after its file in lib/update.
export const update = {
  plugin: {
    incompatibleInstall:
      "The lich plugin in {installs} is not supported by this lich. Install v{version}, the release this lich supports.",
    incompatibleUpdate:
      "The lich plugin in {installs} is not supported by this lich. Update lich, or check that you are online to find a release this lich supports.",
  },
  progress: {
    updateFailed: "Update failed: {error}",
    installFailed: "Install failed: {error}",
    downloadFailed: "Download failed: {error}",
    downloadFailedAt: "Download failed at {percent}%: {error}",
    bytesOf: "{received} of {total}",
  },
} as const
