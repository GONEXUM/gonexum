package main

import (
	"fmt"
	"os"
	"strings"
)

// ensureSettings vérifie que les paramètres requis sont présents.
// Si le fichier de config n'existe pas ou est incomplet, lance le wizard interactif.
// noUpload indique si l'upload est désactivé (dans ce cas, l'API key n'est pas obligatoire).
func ensureSettings(s *Settings, noUpload bool) error {
	path, err := settingsPath()
	if err != nil {
		return fmt.Errorf("impossible de déterminer le chemin de config: %w", err)
	}

	_, statErr := os.Stat(path)
	fileExists := statErr == nil

	needsPasskey := s.Passkey == ""
	needsAPIKey := !noUpload && s.APIKey == ""

	if !fileExists || needsPasskey || needsAPIKey {
		if !fileExists {
			fmt.Printf("  Aucun fichier de configuration trouvé.\n")
			fmt.Printf("  Création : %s\n\n", path)
		} else {
			fmt.Printf("! Configuration incomplète détectée.\n")
			fmt.Printf("  Fichier  : %s\n\n", path)
		}

		runSetupWizard(s, noUpload)

		if err := saveSettings(*s); err != nil {
			return fmt.Errorf("impossible de sauvegarder la configuration: %w", err)
		}
		fmt.Printf("\n  Configuration sauvegardée dans %s\n", path)
	}

	return nil
}

// runSetupWizard demande à l'utilisateur les informations de configuration manquantes.
func runSetupWizard(s *Settings, noUpload bool) {
	fmt.Println("  ╔─────────────────────────────────────────────╗")
	fmt.Println("  ║   Assistant de configuration GONEXUM CLI   ║")
	fmt.Println("  ╚─────────────────────────────────────────────╝")
	fmt.Println()
	fmt.Println("  Ces informations sont disponibles sur votre profil nexum-core.com")
	fmt.Println()

	if s.TrackerURL == "" {
		s.TrackerURL = "https://nexum-core.com"
	}

	// Passkey (toujours requis, intégré dans l'URL announce du torrent)
	if s.Passkey == "" {
		fmt.Println("  Le passkey est intégré dans l'URL announce de chaque torrent créé.")
		for s.Passkey == "" {
			val := readLine("  Passkey nexum-core.com")
			val = strings.TrimSpace(val)
			if val == "" {
				fmt.Println("  ! Le passkey est obligatoire.")
			} else {
				s.Passkey = val
			}
		}
	}

	// API Key (requis sauf si --no-upload)
	if !noUpload && s.APIKey == "" {
		fmt.Println()
		fmt.Println("  L'API key est utilisée pour uploader les torrents.")
		for s.APIKey == "" {
			val := readLine("  API Key nexum-core.com")
			val = strings.TrimSpace(val)
			if val == "" {
				fmt.Println("  ! L'API key est obligatoire (ou utilisez --no-upload).")
			} else {
				s.APIKey = val
			}
		}
	}

	// Dossier de sortie (optionnel)
	fmt.Println()
	fmt.Println("  Dossier où seront enregistrés les fichiers .torrent et .nfo")
	fmt.Println("  (laisser vide = même dossier que la source)")
	val := readLine("  Dossier de sortie")
	val = strings.TrimSpace(val)
	if val != "" {
		if err := os.MkdirAll(val, 0755); err != nil {
			fmt.Printf("  ! Impossible de créer le dossier \"%s\": %v\n", val, err)
			fmt.Println("  Dossier de sortie ignoré.")
		} else {
			s.OutputDir = val
		}
	}

	// URL du tracker (optionnelle, uniquement si différente du défaut)
	fmt.Println()
	current := s.TrackerURL
	val = readLine(fmt.Sprintf("  URL du tracker [défaut: %s]", current))
	val = strings.TrimSpace(val)
	if val != "" && val != current {
		s.TrackerURL = val
	}
}
