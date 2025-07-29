Name:           nl-to-shell
Version:        0.1.0-dev
Release:        1%{?dist}
Summary:        Convert natural language to shell commands using LLMs

License:        MIT
URL:            https://github.com/nl-to-shell/nl-to-shell
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  git
Requires:       git

%description
nl-to-shell is a CLI utility that converts natural language descriptions
into executable shell commands using Large Language Models (LLMs).

It provides context-aware command generation by analyzing your current working
directory, git repository state, files, and other environmental factors.

%prep
%setup -q

%build
# Binary is pre-built

%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT%{_bindir}
install -m 755 nl-to-shell-linux-amd64 $RPM_BUILD_ROOT%{_bindir}/nl-to-shell

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
%{_bindir}/nl-to-shell

%changelog
* Tue Jul 29 2025 nl-to-shell Team <maintainers@nl-to-shell.com> - 0.1.0-dev-1
- Initial package release
