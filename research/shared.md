# shared

The catalog agent `shared` is a pseudo-agent for files and directories read by several agents
at once, so they cannot be attributed to one tool: cross-agent instruction and skill
directories. It also owns the environment variables that move many tools' directories at once
and the shell startup and session environment files where such a variable may be set. It is
collected only, with no parser: these are instruction and skill files, not transcripts.

## Source and evidence

Evidence level: the per-agent documents in this directory, which cite each reader. Each path
below is justified by at least two agents' source. `AGENTS.md` is read by Codex CLI, Amp,
OpenCode, Crush, Factory Droid, Kilo Code and Copilot CLI (see their
documents). `~/.agents` and `~/.config/agents` hold cross-tool skill and
agent definitions adopted by Codex, Copilot CLI and others.

### Observed

- No real install: `shared` is not one agent, and the evidence is each reader's document.

### Source reviews

- 2026-10-19: the Zed review at
  [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
  and the Crush review at
  [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)
  added both as readers of the skills directories ([Per-user storage](#per-user-storage)).

## Collection

### Per-user storage

| Path relative to home | Holds | Readers |
| --- | --- | --- |
| `.agents` | skills and agent definitions in the cross-tool "agents" layout (`skills/<name>/SKILL.md`) | Codex CLI, Copilot CLI, Amp, Zed, Crush, Qwen Code, pi, others |
| `.config/agents` | XDG variant of the same | Codex CLI, Copilot CLI, Amp, Crush, others |
| `.config/AGENTS.md` | user-level instructions | Codex CLI, Amp, OpenCode |

The layout is identical on every OS because the readers use `homedir()`
without platform branching.

Zed reads `~/.agents/skills/<skill>/SKILL.md`
([`crates/agent/src/agent.rs:616-638`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L616-L638)
at
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6);
[Per-user storage in `research/zed.md`](zed.md#per-user-storage)). Crush reads `~/.config/agents/skills`
and `~/.agents/skills`, and also Claude Code's `~/.claude/skills`
([`internal/config/load.go:1312-1322`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/load.go#L1312-L1322)
at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da);
[Per-user storage in `research/crush.md`](crush.md#per-user-storage) and
[Per-user storage in `research/claude-code.md`](claude-code.md#per-user-storage)).

Qwen Code lists `~/.agents/skills` beside its own `~/.qwen/skills`
([`packages/core/src/config/storage.ts:17`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L17)
and
[`754-761`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L754-L761)
at
[`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b);
[Per-user storage in `research/qwen-code.md`](qwen-code.md#per-user-storage)),
and pi treats it as a trusted user skills directory
([`packages/coding-agent/src/core/trust-manager.ts:194-195`](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L194-L195)
at
[`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403);
[Per-user storage in `research/pi.md`](pi.md#per-user-storage)).

#### Scratch directories

None known.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `XDG_CONFIG_HOME` | the parent of every `.config/<tool>` directory an agent or shell reads through it ([XDG Base Directory Specification 0.8, Environment variables](https://specifications.freedesktop.org/basedir/0.8/#variables)) | `.config` | `shared` |
| `XDG_DATA_HOME` | the parent of every `.local/share/<tool>` directory read through it ([XDG Base Directory Specification 0.8, Environment variables](https://specifications.freedesktop.org/basedir/0.8/#variables)) | `.local/share` | `shared` |
| `XDG_STATE_HOME` | the parent of every `.local/state/<tool>` directory read through it ([XDG Base Directory Specification 0.8, Environment variables](https://specifications.freedesktop.org/basedir/0.8/#variables)) | `.local/state` | `shared` |
| `XDG_CACHE_HOME` | the parent of every `.cache/<tool>` directory read through it ([XDG Base Directory Specification 0.8, Environment variables](https://specifications.freedesktop.org/basedir/0.8/#variables)) | `.cache` | `shared` |
| `ZDOTDIR` | the directory zsh reads `.zshenv`, `.zprofile`, `.zshrc` and `.zlogin` from ([`Src/init.c:1715-1735`](https://github.com/zsh-users/zsh/blob/30e8be913521523d226789e53a4a2202dabff61c/Src/init.c#L1715-L1735)) | | `shared` |
| `ENV` | the file an interactive POSIX sh, dash, ksh or bash in POSIX mode reads after the profiles ([`src/main.c:158-167`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/main.c#L158-L167), [bash `shell.c:1234-1245`](https://git.savannah.gnu.org/cgit/bash.git/tree/shell.c?id=9c465866b7d849378369ef700cbe2965aa9691e3#n1234)) | | `shared` |
| `BASH_ENV` | the file a non-interactive bash reads before its script ([bash `shell.c:1210-1219`](https://git.savannah.gnu.org/cgit/bash.git/tree/shell.c?id=9c465866b7d849378369ef700cbe2965aa9691e3#n1210)) | | `shared` |
| `ZSH_CUSTOM` | Oh My Zsh's custom directory, whose `*.zsh` files it sources ([`oh-my-zsh.sh:55`](https://github.com/ohmyzsh/ohmyzsh/blob/b509b1c6ed0c1ba37de12cbd37f30b2b2f61193a/oh-my-zsh.sh#L55), [`oh-my-zsh.sh:322-324`](https://github.com/ohmyzsh/ohmyzsh/blob/b509b1c6ed0c1ba37de12cbd37f30b2b2f61193a/oh-my-zsh.sh#L322-L324)) | | `shared` |
| `BASH_IT_CUSTOM` | Bash-it's custom directory, whose `*.bash` and `*/*.bash` files it sources ([`bash_it.sh:15`](https://github.com/Bash-it/bash-it/blob/72ea9e285fe1828977d1cd2bc596b901bcc57440/bash_it.sh#L15), [`bash_it.sh:91-98`](https://github.com/Bash-it/bash-it/blob/72ea9e285fe1828977d1cd2bc596b901bcc57440/bash_it.sh#L91-L98)) | | `shared` |
| `OMF_CONFIG` | Oh My Fish's configuration directory, holding `before.init.fish` and `init.fish` ([`init.fish:1-8`](https://github.com/oh-my-fish/oh-my-fish/blob/043699af326b1d4fcfaf1b2e591d830d1d53506a/init.fish#L1-L8)) | | `shared` |

The XDG variables (named after the freedesktop.org Cross-Desktop Group) belong here because each
moves the directories of many tools at once; an agent's own documents name the agents that honour
them. `ENV`, `BASH_ENV`, `ZDOTDIR` and the framework variables move no agent store: they move the
startup files listed below, so a value found for one of them names more files to search.

#### Startup and environment files

Shells, desktop sessions and developer tools read these files to set the environment of every
program a user starts, so a relocation variable set in one moves an agent's store for that user.
The collector only searches them for the variable names; it never runs or evaluates them.

Shells of the sh family (bash, zsh, the POSIX sh implementations and the Korn shells):

| Shell or mechanism | Files searched | Scope | Source |
| --- | --- | --- | --- |
| bash | `.bash_profile`, `.bash_login`, `.profile`, `.bashrc`, `.bashrc.d/*` | `home` | [`shell.c:1116-1127`](https://git.savannah.gnu.org/cgit/bash.git/tree/shell.c?id=9c465866b7d849378369ef700cbe2965aa9691e3#n1116), [`config-top.h:92`](https://git.savannah.gnu.org/cgit/bash.git/tree/config-top.h?id=9c465866b7d849378369ef700cbe2965aa9691e3#n92); `.bashrc.d/*` from the Fedora skeleton [`dot-bashrc` lines 17-23](https://src.fedoraproject.org/rpms/bash/raw/089036c4d025cc87efd1d84dd0a2ca32383eb120/f/dot-bashrc) |
| bash | `/etc/profile`, `/etc/profile.d/*.sh`, `/etc/profile.d/sh.local`, `/etc/bash.bashrc`, `/etc/bashrc` | `system` | `/etc/profile`: [`shell.c:1116-1127`](https://git.savannah.gnu.org/cgit/bash.git/tree/shell.c?id=9c465866b7d849378369ef700cbe2965aa9691e3#n1116); `/etc/profile.d/*.sh`: Debian [`share/profile` lines 27-34](https://sources.debian.org/src/base-files/14.2/share/profile/#L27-L34), Fedora and its `sh.local` [`profile` line 64](https://src.fedoraproject.org/rpms/setup/raw/5da28d20eab4728d788c70abb0d51551eb528627/f/profile); `/etc/bash.bashrc`: Debian [`deb-bash-config.diff` line 33](https://sources.debian.org/src/bash/5.2.37-2/debian/patches/deb-bash-config.diff/#L33), Arch [`PKGBUILD` line 108](https://gitlab.archlinux.org/archlinux/packaging/packages/bash/-/blob/45f3f3f99f3211563f70fe8018429c90caaf9e03/PKGBUILD#L108); `/etc/bashrc`: Fedora [`dot-bashrc` lines 3-6](https://src.fedoraproject.org/rpms/bash/raw/089036c4d025cc87efd1d84dd0a2ca32383eb120/f/dot-bashrc) |
| zsh | `.zshenv`, `.zprofile`, `.zshrc`, `.zlogin` | `home` | [`Src/init.c:1507-1551`](https://github.com/zsh-users/zsh/blob/30e8be913521523d226789e53a4a2202dabff61c/Src/init.c#L1507-L1551), [`Doc/Zsh/files.yo:14-40`](https://github.com/zsh-users/zsh/blob/30e8be913521523d226789e53a4a2202dabff61c/Doc/Zsh/files.yo#L14-L40) |
| zsh | `/etc/zshenv`, `/etc/zprofile`, `/etc/zshrc`, `/etc/zlogin` | `system` | upstream default [`configure.ac:153-171`](https://github.com/zsh-users/zsh/blob/30e8be913521523d226789e53a4a2202dabff61c/configure.ac#L153-L171), Fedora [`zsh.spec` line 80](https://src.fedoraproject.org/rpms/zsh/raw/9ba42af1aba892a9214a8471b1b27ff0859c6152/f/zsh.spec), Homebrew [`zsh.rb:70`](https://github.com/Homebrew/homebrew-core/blob/efbf87049c0041777d40f865addf9a68ed95e1cb/Formula/z/zsh.rb#L70) |
| zsh | `/etc/zsh/zshenv`, `/etc/zsh/zprofile`, `/etc/zsh/zshrc`, `/etc/zsh/zlogin` | `system` | Debian [`debian/rules` line 24](https://sources.debian.org/src/zsh/5.9-8/debian/rules/#L24), Arch [`PKGBUILD` lines 52-57](https://gitlab.archlinux.org/archlinux/packaging/packages/zsh/-/blob/0ba28c49a4cc4fb9baaa178edaac8e24f12abbbe/PKGBUILD#L52-L57) |
| zsh | `/usr/local/etc/zshenv`, `/usr/local/etc/zprofile`, `/usr/local/etc/zshrc`, `/usr/local/etc/zlogin` | `system` | FreeBSD port [`shells/zsh/Makefile:62`](https://github.com/freebsd/freebsd-ports/blob/587b444789986f27b6feaba29a09f70144aafa05/shells/zsh/Makefile#L62) |
| POSIX sh, dash, busybox ash, FreeBSD sh | `.profile`, `.shrc`, `.shinit` | `home` | dash [`src/main.c:149-155`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/main.c#L149-L155), busybox [`shell/ash.c:14757-14777`](https://github.com/mirror/busybox/blob/371fe9f71d445d18be28c82a2a6d82115c8af19d/shell/ash.c#L14757-L14777), FreeBSD [`bin/sh/main.c:157-172`](https://github.com/freebsd/freebsd-src/blob/b138d017c70421e9d074096fcc3ac676f7e1c846/bin/sh/main.c#L157-L172); `.shrc` as the `ENV` file of the FreeBSD skeleton [`share/skel/dot.profile:20`](https://github.com/freebsd/freebsd-src/blob/b138d017c70421e9d074096fcc3ac676f7e1c846/share/skel/dot.profile#L20); `.shinit` as the `ENV` file of the dash manual [`src/dash.1:157`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/dash.1#L157) |
| POSIX sh, dash, busybox ash, FreeBSD sh | `/etc/profile`, `/etc/profile.d/*.sh`, `/etc/suid_profile` | `system` | dash [`src/main.c:149-155`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/main.c#L149-L155); `/etc/profile.d/*.sh`: Debian [`share/profile` lines 27-34](https://sources.debian.org/src/base-files/14.2/share/profile/#L27-L34), Alpine [`alpine-baselayout/profile:19-23`](https://github.com/alpinelinux/aports/blob/7749273fed55f6e1df7c9ee6a127f18099f98a94/main/alpine-baselayout/profile#L19-L23); `/etc/suid_profile` for a privileged FreeBSD sh [`bin/sh/main.c:157-172`](https://github.com/freebsd/freebsd-src/blob/b138d017c70421e9d074096fcc3ac676f7e1c846/bin/sh/main.c#L157-L172) |
| ksh93 | `.profile`, `.kshrc` | `home` | [`data/msg.c:177`](https://github.com/ksh93/ksh/blob/3322229017073b0b96c5ebe16c21d18644e252a9/src/cmd/ksh93/data/msg.c#L177); `.kshrc` as the default `ENV` [`sh/init.c:1954-1958`](https://github.com/ksh93/ksh/blob/3322229017073b0b96c5ebe16c21d18644e252a9/src/cmd/ksh93/sh/init.c#L1954-L1958) |
| ksh93 | `/etc/profile`, `/etc/ksh.kshrc`, `/etc/suid_profile` | `system` | [`data/msg.c:177-186`](https://github.com/ksh93/ksh/blob/3322229017073b0b96c5ebe16c21d18644e252a9/src/cmd/ksh93/data/msg.c#L177-L186), [`sh/main.c:178-203`](https://github.com/ksh93/ksh/blob/3322229017073b0b96c5ebe16c21d18644e252a9/src/cmd/ksh93/sh/main.c#L178-L203), [`SHOPT.sh:34`](https://github.com/ksh93/ksh/blob/3322229017073b0b96c5ebe16c21d18644e252a9/src/cmd/ksh93/SHOPT.sh#L34) |
| mksh | `.profile`, `.mkshrc` | `home` | [`main.c:37`](https://github.com/MirBSD/mksh/blob/341752823184238dc9b2b0a22b48da984f25442d/main.c#L37), [`main.c:744-763`](https://github.com/MirBSD/mksh/blob/341752823184238dc9b2b0a22b48da984f25442d/main.c#L744-L763) |
| mksh | `/etc/profile`, `/etc/suid_profile` | `system` | [`sh.h:1804-1809`](https://github.com/MirBSD/mksh/blob/341752823184238dc9b2b0a22b48da984f25442d/sh.h#L1804-L1809) |
| OpenBSD ksh | `.profile`, `.kshrc` | `home` | [`bin/ksh/main.c:406-428`](https://github.com/openbsd/src/blob/928141eb6cf7644d7b572d51fb83c69fb6dbee3f/bin/ksh/main.c#L406-L428); `.kshrc` only through `ENV` [`bin/ksh/ksh.1:175-178`](https://github.com/openbsd/src/blob/928141eb6cf7644d7b572d51fb83c69fb6dbee3f/bin/ksh/ksh.1#L175-L178) |
| OpenBSD ksh | `/etc/profile`, `/etc/suid_profile`, `/etc/ksh.kshrc` | `system` | [`bin/ksh/main.c:406-428`](https://github.com/openbsd/src/blob/928141eb6cf7644d7b572d51fb83c69fb6dbee3f/bin/ksh/main.c#L406-L428); `/etc/ksh.kshrc` is read only when a profile sources it [`bin/ksh/ksh.1:5537-5542`](https://github.com/openbsd/src/blob/928141eb6cf7644d7b572d51fb83c69fb6dbee3f/bin/ksh/ksh.1#L5537-L5542) |

Other shells:

| Shell or mechanism | Files searched | Scope | Source |
| --- | --- | --- | --- |
| fish | `.config/fish/config.fish`, `.config/fish/conf.d/*.fish`, `.config/fish/fish_variables`, `.config/fish/fishd.*`, `.local/share/fish/vendor_conf.d/*.fish` | `home` | [`doc_src/language.rst:2003-2037`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/doc_src/language.rst#L2003-L2037), [`share/config.fish:37`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/share/config.fish#L37); `fish_variables` [`doc_src/language.rst:1322`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/doc_src/language.rst#L1322); `fishd.<machine id>` before fish 3.0 [`src/env_universal_common.cpp:81-88`](https://github.com/fish-shell/fish-shell/blob/b680db8ce683db285c750ae5af534c97d08b1b40/src/env_universal_common.cpp#L81-L88) (2.7.1), renamed in [`CHANGELOG.rst:3497-3499`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/CHANGELOG.rst#L3497-L3499) |
| fish | `/etc/fish/config.fish`, `/etc/fish/conf.d/*.fish`, `/usr/share/fish/vendor_conf.d/*.fish`, `/usr/local/share/fish/vendor_conf.d/*.fish` | `system` | [`doc_src/language.rst:2003-2037`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/doc_src/language.rst#L2003-L2037), [`build.rs:165-168`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/build.rs#L165-L168) |
| fish | `/usr/local/etc/fish/config.fish`, `/usr/local/etc/fish/conf.d/*.fish` | `system` | FreeBSD port [`shells/fish/pkg-plist:4`](https://github.com/freebsd/freebsd-ports/blob/587b444789986f27b6feaba29a09f70144aafa05/shells/fish/pkg-plist#L4), [`pkg-plist:1521`](https://github.com/freebsd/freebsd-ports/blob/587b444789986f27b6feaba29a09f70144aafa05/shells/fish/pkg-plist#L1521) |
| fish | `/opt/homebrew/etc/fish/config.fish`, `/opt/homebrew/etc/fish/conf.d/*.fish`, `/opt/homebrew/share/fish/vendor_conf.d/*.fish` | `system` | Homebrew [`fish.rb:37`](https://github.com/Homebrew/homebrew-core/blob/efbf87049c0041777d40f865addf9a68ed95e1cb/Formula/f/fish.rb#L37), [`fish.rb:41`](https://github.com/Homebrew/homebrew-core/blob/efbf87049c0041777d40f865addf9a68ed95e1cb/Formula/f/fish.rb#L41), with the `/opt/homebrew` prefix of [Homebrew Installation](https://docs.brew.sh/Installation) |
| csh, tcsh | `.tcshrc`, `.cshrc`, `.login` | `home` | [`tcsh.man.in:277-330`](https://github.com/tcsh-org/tcsh/blob/b7db07931546178c4cb2542b5fc850c483628ed5/tcsh.man.in#L277-L330) |
| csh, tcsh | `/etc/csh.cshrc`, `/etc/csh.login`, `/etc/.cshrc`, `/etc/.login` | `system` | [`pathnames.h:122-128`](https://github.com/tcsh-org/tcsh/blob/b7db07931546178c4cb2542b5fc850c483628ed5/pathnames.h#L122-L128); the dot forms on Solaris [`pathnames.h:68-78`](https://github.com/tcsh-org/tcsh/blob/b7db07931546178c4cb2542b5fc850c483628ed5/pathnames.h#L68-L78) |
| Nushell | `.config/nushell/env.nu`, `.config/nushell/config.nu`, `.config/nushell/login.nu`, `.config/nushell/autoload/*.nu`, `Library/Application Support/nushell/env.nu`, `Library/Application Support/nushell/config.nu`, `Library/Application Support/nushell/login.nu`, `Library/Application Support/nushell/autoload/*.nu`, `AppData/Roaming/nushell/env.nu`, `AppData/Roaming/nushell/config.nu`, `AppData/Roaming/nushell/login.nu`, `AppData/Roaming/nushell/autoload/*.nu`, `.local/share/nushell/vendor/autoload/*.nu` | `home` | [`book/configuration.md:70-89`](https://github.com/nushell/nushell.github.io/blob/714dcbdaf9d3e0832e9963eb9b9efec8382827fc/book/configuration.md#L70-L89), [`book/configuration.md:425-429`](https://github.com/nushell/nushell.github.io/blob/714dcbdaf9d3e0832e9963eb9b9efec8382827fc/book/configuration.md#L425-L429), [`crates/nu-config/src/resolve.rs:59`](https://github.com/nushell/nushell/blob/e6b30c87a9595ff7e4f53f8380f5d683ed1b56f9/crates/nu-config/src/resolve.rs#L59) |
| Nushell | `/usr/share/nushell/vendor/autoload/*.nu`, `/usr/local/share/nushell/vendor/autoload/*.nu`, `/Library/Application Support/nushell/vendor/autoload/*.nu`, `/ProgramData/nushell/vendor/autoload/*.nu` | `system` | [`crates/nu-config/src/resolve.rs:192-275`](https://github.com/nushell/nushell/blob/e6b30c87a9595ff7e4f53f8380f5d683ed1b56f9/crates/nu-config/src/resolve.rs#L192-L275) |
| xonsh | `.xonshrc`, `.config/xonsh/rc.xsh`, `.config/xonsh/rc.d/*.xsh`, `.config/xonsh/rc.d/*.py` | `home` | [`docs/xonshrc.rst:20-23`](https://github.com/xonsh/xonsh/blob/b3c9e1e34458b7464ad9c3ba4655f89c09d95d34/docs/xonshrc.rst#L20-L23), [`xonsh/environ.py:819-860`](https://github.com/xonsh/xonsh/blob/b3c9e1e34458b7464ad9c3ba4655f89c09d95d34/xonsh/environ.py#L819-L860) |
| xonsh | `/etc/xonsh/xonshrc`, `/etc/xonsh/rc.d/*.xsh`, `/etc/xonsh/rc.d/*.py` | `system` | [`xonsh/environ.py:800-809`](https://github.com/xonsh/xonsh/blob/b3c9e1e34458b7464ad9c3ba4655f89c09d95d34/xonsh/environ.py#L800-L809), [`docs/xonshrc.rst:20-23`](https://github.com/xonsh/xonsh/blob/b3c9e1e34458b7464ad9c3ba4655f89c09d95d34/docs/xonshrc.rst#L20-L23) |
| elvish | `.config/elvish/rc.elv`, `.elvish/rc.elv`, `AppData/Roaming/elvish/rc.elv` | `home` | [`website/ref/command.md:22-34`](https://github.com/elves/elvish/blob/26a8bd5c4ee1eb5c0a2d53578d0368de2b8b3274/website/ref/command.md#L22-L34), [`pkg/shell/paths.go:13-21`](https://github.com/elves/elvish/blob/26a8bd5c4ee1eb5c0a2d53578d0368de2b8b3274/pkg/shell/paths.go#L13-L21), [`pkg/shell/paths_windows.go:13`](https://github.com/elves/elvish/blob/26a8bd5c4ee1eb5c0a2d53578d0368de2b8b3274/pkg/shell/paths_windows.go#L13) |
| PowerShell | `Documents/WindowsPowerShell/*.ps1`, `Documents/PowerShell/*.ps1`, `.config/powershell/*.ps1` | `home` | 5.1 [`about_Profiles.md:37-41`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/5.1/Microsoft.PowerShell.Core/About/about_Profiles.md#L37-L41), 7.5 [`about_Profiles.md:36-52`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/7.5/Microsoft.PowerShell.Core/About/about_Profiles.md#L36-L52) |
| PowerShell | `/Windows/System32/WindowsPowerShell/v1.0/*.ps1`, `/Program Files/PowerShell/7/*.ps1`, `/opt/microsoft/powershell/7/*.ps1`, `/usr/local/microsoft/powershell/7/*.ps1` | `system` | 7.5 [`about_Profiles.md:36-52`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/7.5/Microsoft.PowerShell.Core/About/about_Profiles.md#L36-L52), [`about_Automatic_Variables.md:587-588`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/7.5/Microsoft.PowerShell.Core/About/about_Automatic_Variables.md#L587-L588); the 5.1 `$PSHOME` from the example output of [`about_Properties.md:138-139`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/5.1/Microsoft.PowerShell.Core/About/about_Properties.md#L138-L139) |

A Windows profile follows the Documents known folder, not `$HOME\Documents`
([`CorePsPlatform.cs:174-176`](https://github.com/PowerShell/PowerShell/blob/01b9a93c3a20a36c4ab8e4c0a54e61ab9d921a64/src/System.Management.Automation/CoreCLR/CorePsPlatform.cs#L174-L176),
[`about_Profiles.md:74-79`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/7.5/Microsoft.PowerShell.Core/About/about_Profiles.md#L74-L79)),
so OneDrive or folder redirection puts it elsewhere; no glob is listed for those folders because
their names are not documented.

Session environments, read when a user logs in or a desktop session starts:

| Shell or mechanism | Files searched | Scope | Source |
| --- | --- | --- | --- |
| PAM `pam_env` (Pluggable Authentication Modules) | `/etc/environment`, `/etc/security/pam_env.conf` | `system` | [`pam_env.c:46-57`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/modules/pam_env/pam_env.c#L46-L57), [`pam_env.conf.5.xml:18-33`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/modules/pam_env/pam_env.conf.5.xml#L18-L33) |
| PAM `pam_env` | `.pam_environment` | `home` | [`pam_env.c:52-53`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/modules/pam_env/pam_env.c#L52-L53), read only with `user_readenv=1`, off by default since 1.4.0 ([`NEWS:181-189`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/NEWS#L181-L189)) and deprecated ([`pam_env.8.xml:209-222`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/modules/pam_env/pam_env.8.xml#L209-L222)) |
| systemd `environment.d` | `.config/environment.d/*.conf` | `home` | [`man/environment.d.xml:30-34`](https://github.com/systemd/systemd/blob/de3a0977e17cb7366d22cc236fde42ed6818c696/man/environment.d.xml#L30-L34) |
| systemd `environment.d` | `/etc/environment.d/*.conf`, `/run/environment.d/*.conf`, `/usr/local/lib/environment.d/*.conf`, `/usr/lib/environment.d/*.conf` | `system` | [`man/environment.d.xml:30-34`](https://github.com/systemd/systemd/blob/de3a0977e17cb7366d22cc236fde42ed6818c696/man/environment.d.xml#L30-L34) |
| KDE Plasma | `.config/plasma-workspace/env/*.sh` | `home` | [`startkde/startplasma.cpp:308-340`](https://github.com/KDE/plasma-workspace/blob/298f9dd7dfcebd92a0a3b82101415c65afeee6b6/startkde/startplasma.cpp#L308-L340) |
| KDE Plasma | `/etc/xdg/plasma-workspace/env/*.sh` | `system` | [`startkde/startplasma.cpp:308-340`](https://github.com/KDE/plasma-workspace/blob/298f9dd7dfcebd92a0a3b82101415c65afeee6b6/startkde/startplasma.cpp#L308-L340), with `/etc/xdg` as the default system configuration directory of the [XDG Base Directory Specification 0.8, Environment variables](https://specifications.freedesktop.org/basedir/0.8/#variables) |
| X session (Debian family) | `.xsessionrc` | `home` | [`Xsession` line 59](https://sources.debian.org/src/xorg/1:7.7+24+deb13u1/debian/local/Xsession/#L59), [`40x11-common_xsessionrc` lines 3-6](https://sources.debian.org/src/xorg/1:7.7+24+deb13u1/debian/local/Xsession.d/40x11-common_xsessionrc/#L3-L6) |
| macOS launchd (the service manager) | `Library/LaunchAgents/*.plist` | `home` | the `EnvironmentVariables` key and FILES of [launchd.plist(5)](https://keith.github.io/xcode-man-pages/launchd.plist.5.html) (Xcode manual page mirror) |
| macOS launchd | `/Library/LaunchAgents/*.plist`, `/Library/LaunchDaemons/*.plist` | `system` | FILES of [launchd.plist(5)](https://keith.github.io/xcode-man-pages/launchd.plist.5.html) |

Project tools and shell frameworks:

| Shell or mechanism | Files searched | Scope | Source |
| --- | --- | --- | --- |
| direnv | `.envrc`, `.env` | `project` | [`internal/cmd/rc.go:420-425`](https://github.com/direnv/direnv/blob/3d0b4ae1ed83064a10ada052949596e7463a8768/internal/cmd/rc.go#L420-L425); `.env` only with `load_dotenv` set ([`man/direnv.toml.1.md:45-49`](https://github.com/direnv/direnv/blob/3d0b4ae1ed83064a10ada052949596e7463a8768/man/direnv.toml.1.md#L45-L49)) |
| direnv | `.config/direnv/direnvrc`, `.config/direnv/direnv.toml`, `.config/direnv/lib/*.sh` | `home` | [`man/direnv.1.md:222-229`](https://github.com/direnv/direnv/blob/3d0b4ae1ed83064a10ada052949596e7463a8768/man/direnv.1.md#L222-L229) |
| mise | `mise.toml`, `.mise.toml`, `mise.local.toml`, `.mise.local.toml`, `mise/config.toml`, `mise/conf.d/*.toml`, `.mise/config.toml`, `.mise/conf.d/*.toml`, `.config/mise.toml`, `.config/mise/mise.toml`, `.config/mise/config.toml`, `.config/mise/conf.d/*.toml` | `project` | [`docs/configuration.md:127-152`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/configuration.md#L127-L152) |
| mise | `.config/mise/config.toml`, `.config/mise/conf.d/*.toml` | `home` | [`docs/configuration.md:240-246`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/configuration.md#L240-L246), [`docs/configuration.md:176-181`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/configuration.md#L176-L181) |
| mise | `/etc/mise/config.toml`, `/etc/mise/conf.d/*.toml` | `system` | [`docs/configuration.md:271-275`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/configuration.md#L271-L275), [`docs/configuration.md:176-181`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/configuration.md#L176-L181) |
| Oh My Zsh | `.oh-my-zsh/custom/*.zsh` | `home` | [`oh-my-zsh.sh:55`](https://github.com/ohmyzsh/ohmyzsh/blob/b509b1c6ed0c1ba37de12cbd37f30b2b2f61193a/oh-my-zsh.sh#L55), [`oh-my-zsh.sh:322-324`](https://github.com/ohmyzsh/ohmyzsh/blob/b509b1c6ed0c1ba37de12cbd37f30b2b2f61193a/oh-my-zsh.sh#L322-L324), [`templates/zshrc.zsh-template:5`](https://github.com/ohmyzsh/ohmyzsh/blob/b509b1c6ed0c1ba37de12cbd37f30b2b2f61193a/templates/zshrc.zsh-template#L5) |
| Bash-it | `.bash_it/custom/*.bash`, `.bash_it/custom/*/*.bash` | `home` | [`bash_it.sh:91-98`](https://github.com/Bash-it/bash-it/blob/72ea9e285fe1828977d1cd2bc596b901bcc57440/bash_it.sh#L91-L98), [`docs/installation.rst:8`](https://github.com/Bash-it/bash-it/blob/72ea9e285fe1828977d1cd2bc596b901bcc57440/docs/installation.rst#L8) |
| Oh My Fish | `.config/omf/before.init.fish`, `.config/omf/init.fish` | `home` | [`init.fish:1-8`](https://github.com/oh-my-fish/oh-my-fish/blob/043699af326b1d4fcfaf1b2e591d830d1d53506a/init.fish#L1-L8), [`init.fish:49`](https://github.com/oh-my-fish/oh-my-fish/blob/043699af326b1d4fcfaf1b2e591d830d1d53506a/init.fish#L49), [`lib/require.fish:33`](https://github.com/oh-my-fish/oh-my-fish/blob/043699af326b1d4fcfaf1b2e591d830d1d53506a/lib/require.fish#L33) |

A responder reading a matched line will see the variable set in the syntax of its file:
`export NAME=value`
([`src/dash.1:1248-1251`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/dash.1#L1248-L1251))
or `NAME=value; export NAME`
([`src/dash.1:157`](https://github.com/tklauser/dash/blob/057cd650a4edd5856213d431a974ff35c6594489/src/dash.1#L157))
in the sh family; `set -gx NAME value`
([`doc_src/language.rst:411`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/doc_src/language.rst#L411))
in fish scripts and `SETUVAR --export NAME:value`
([`src/env_universal_common.rs:730-741`](https://github.com/fish-shell/fish-shell/blob/d92ccd4f8f6f08cfc8a0d9fa17333153e2922af1/src/env_universal_common.rs#L730-L741))
in `fish_variables`; `setenv NAME value`
([`tcsh.man.in:6627`](https://github.com/tcsh-org/tcsh/blob/b7db07931546178c4cb2542b5fc850c483628ed5/tcsh.man.in#L6627))
in csh; `$env.NAME = value`
([`book/environment.md:37-42`](https://github.com/nushell/nushell.github.io/blob/714dcbdaf9d3e0832e9963eb9b9efec8382827fc/book/environment.md#L37-L42))
in Nushell; `$NAME = value`
([`docs/bash_to_xsh.rst:41-43`](https://github.com/xonsh/xonsh/blob/b3c9e1e34458b7464ad9c3ba4655f89c09d95d34/docs/bash_to_xsh.rst#L41-L43))
in xonsh; `set E:NAME = value`
([`website/learn/tour.md:410`](https://github.com/elves/elvish/blob/26a8bd5c4ee1eb5c0a2d53578d0368de2b8b3274/website/learn/tour.md#L410))
in elvish; `$env:NAME = value`
([`about_Environment_Variables.md:99-101`](https://github.com/MicrosoftDocs/PowerShell-Docs/blob/ebaa235784a688ec72ab642186c4c17460fb438b/reference/7.5/Microsoft.PowerShell.Core/About/about_Environment_Variables.md#L99-L101)),
in any letter case, in PowerShell; `KEY=VAL` lines
([`modules/pam_env/environment:4`](https://github.com/linux-pam/linux-pam/blob/584ef2275254016ee68126f90dab6df3604124b9/modules/pam_env/environment#L4))
in `/etc/environment` and the `environment.d` files; `<key>NAME</key>` inside the
`EnvironmentVariables` dictionary of a launchd plist
([launchd.plist(5)](https://keith.github.io/xcode-man-pages/launchd.plist.5.html));
and `NAME = "value"` under an `[env]` table in mise
([`docs/environments/index.md:18-21`](https://github.com/jdx/mise/blob/02d59ef93faf219319cccac23249501e8763e2da/docs/environments/index.md#L18-L21)).

### Credentials

None. The home-level and project-level `.env` files that Aider, Gemini
CLI, Qwen Code and Continue read for API keys are general-purpose
environment files and are not collected (collector 1.7.0); they are left to
the EDR. Agent-specific `.env` files inside an agent's own directory, such
as `.gemini/.env` and `.qwen/.env`, are collected under that agent.

### Exclusions

None; these are small text files.

### Project-local files

`AGENTS.md` at the project root, collected through the project catalog.

### Where the project path is recorded

Not applicable.

## Parsing

`shared` is `collected` only: there is nothing to parse. Its paths are cross-agent instruction
and skill directories (`.agents`, `.config/agents`, `.config/AGENTS.md`) with no transcripts,
written by the user and read by several agents, so they record what an agent was told, not what
it did. The transcripts of each reader are parsed, where they are, under that agent.

## Confidence

- High for every path (each reader's source).
- Attribution is the one judgement: a `shared` match says several agents may have read the file,
  not which one did.
- Not determined for the startup file list: which display managers read `.xprofile`, whether
  SUSE reads `/etc/bash.bashrc.local`, the Windows PowerShell 5.1 `$PSHOME` path (seen only in
  example output), and the folder names OneDrive gives a redirected Documents folder.
