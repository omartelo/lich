import { describe, expect, it } from "vitest"
import { accountLabel, splitAccount, upgradeAccount } from "./gh-account"

describe("upgradeAccount", () => {
  const accounts = ["github.com/omartelo", "ghe.example.com/work-login"]

  it("upgrades a host-less account gh knows on exactly one host", () => {
    expect(upgradeAccount("omartelo", accounts)).toBe("github.com/omartelo")
  })

  it("leaves an account that already names its host alone", () => {
    expect(upgradeAccount("github.com/omartelo", accounts)).toBeNull()
  })

  it("refuses to guess when the login exists on two hosts", () => {
    expect(
      upgradeAccount("omartelo", ["github.com/omartelo", "ghe.example.com/omartelo"]),
    ).toBeNull()
  })

  it("leaves a login gh no longer knows alone, so it stays visible and changeable", () => {
    expect(upgradeAccount("gone", accounts)).toBeNull()
  })

  it("has nothing to do for no account at all", () => {
    expect(upgradeAccount("", accounts)).toBeNull()
  })
})

describe("splitAccount", () => {
  it("splits host from login", () => {
    expect(splitAccount("github.com/omartelo")).toEqual(["github.com", "omartelo"])
  })

  it("reads an account stored before hosts travelled with them", () => {
    expect(splitAccount("omartelo")).toEqual(["", "omartelo"])
  })

  it("keeps a login containing a slash on the login side", () => {
    expect(splitAccount("ghe.example.com/team/bot")).toEqual(["ghe.example.com", "team/bot"])
  })
})

describe("accountLabel", () => {
  const single = ["github.com/omartelo", "github.com/marcelo-filho_snk"]
  const mixed = ["github.com/omartelo", "ghe.example.com/omartelo"]

  it("leaves the host out when every account shares github.com", () => {
    expect(accountLabel("github.com/omartelo", single)).toBe("omartelo")
  })

  it("names the host once two of them are in play", () => {
    expect(accountLabel("github.com/omartelo", mixed)).toBe("omartelo — github.com")
    expect(accountLabel("ghe.example.com/omartelo", mixed)).toBe("omartelo — ghe.example.com")
  })

  it("names an enterprise host even when it is the only one", () => {
    const only = ["ghe.example.com/work-login"]
    expect(accountLabel("ghe.example.com/work-login", only)).toBe("work-login — ghe.example.com")
  })

  it("shows a host-less account as its bare login", () => {
    expect(accountLabel("omartelo", ["omartelo"])).toBe("omartelo")
  })
})
