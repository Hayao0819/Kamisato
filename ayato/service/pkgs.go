package service

// PkgFiles はリポジトリ内のパッケージファイル一覧を返します。
func (s *Service) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	files, err := s.pkgBinaryRepo.PkgFiles(repoName, archName, pkgName)
	if err != nil {
		return nil, err
	}
	return files, nil
}
