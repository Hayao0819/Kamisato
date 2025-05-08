package repo

func (r *SourceRepo) UploadAllPackageToBlinky(server string) error {
	for _, pkg := range r.Pkgs {
		if err := pkg.UploadToBlinky(server, r); err != nil {
			return err
		}
	}
	return nil
}
