export interface PackageInfo {
  pkgname: string;
  pkgbase: string;
  pkgver: string;
  pkgdesc: string;
  url: string;
  builddate: number;
  packager: string;
  size: number;
  arch: string;
  license: string[];
  replaces: string[];
  group: string[];
  conflict: string[];
  provides: string[];
  backup: string[];
  depend: string[];
  optdepend: string[];
  makedepend: string[];
  checkdepend: string[];
  xdata: { [key: string]: string };
  pkgtype: string;
}

export interface PacmanPkgsResponse {
  name: string;
  arch: string;
  packages: PackageInfo[];
}

// TODO: Add other relevant type definitions from the backend
