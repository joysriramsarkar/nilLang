package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/joysriramsarkar/nilLang/pkg/signing"
	"golang.org/x/term"
)

func getKeystorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".nilang")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "keystore.json")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate", "gen":
		cmdGenerate()
	case "list", "ls":
		cmdList()
	case "sign":
		cmdSign()
	case "verify":
		cmdVerify()
	case "export":
		cmdExport()
	case "delete", "rm":
		cmdDelete()
	case "version", "-v", "--version":
		fmt.Println("nilkey v0.1.0 - Nilang Key Management Tool")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "❌ অজানা কমান্ড: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("nilkey - Nilang Key Management Tool")
	fmt.Println()
	fmt.Println("ব্যবহার: nilkey <কমান্ড> [অপশন]")
	fmt.Println()
	fmt.Println("কমান্ড:")
	fmt.Println("  generate              নতুন কী পেয়ার তৈরি করুন")
	fmt.Println("  list                  সব কী তালিকা দেখুন")
	fmt.Println("  sign <file.nilax>     প্যাকেজ সাইন করুন")
	fmt.Println("  verify <file.nilax>   প্যাকেজ সিগনেচার ভেরিফাই করুন")
	fmt.Println("  export <key-id>       পাবলিক কী এক্সপোর্ট করুন")
	fmt.Println("  delete <key-id>       কী মুছে ফেলুন")
	fmt.Println("  version               ভার্সন তথ্য দেখুন")
	fmt.Println("  help                  এই সাহায্য বার্তা দেখুন")
}

func cmdGenerate() {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	nameFlag := fs.String("name", "", "Developer Name")
	emailFlag := fs.String("email", "", "Email")
	passFlag := fs.String("password", "", "Password")
	fs.Parse(os.Args[2:])

	owner := *nameFlag
	email := *emailFlag
	password := *passFlag

	reader := bufio.NewReader(os.Stdin)

	if owner == "" {
		fmt.Print("👤 আপনার নাম: ")
		owner, _ = reader.ReadString('\n')
		owner = strings.TrimSpace(owner)
	}

	if email == "" {
		fmt.Print("📧 ইমেইল: ")
		email, _ = reader.ReadString('\n')
		email = strings.TrimSpace(email)
	}

	if password == "" {
		fmt.Print("🔑 কীস্টোর পাসওয়ার্ড: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ পাসওয়ার্ড পড়তে সমস্যা: %s\n", err)
			os.Exit(1)
		}
		password = string(passwordBytes)
		fmt.Println()

		fmt.Print("🔑 পাসওয়ার্ড নিশ্চিত করুন: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ পাসওয়ার্ড পড়তে সমস্যা: %s\n", err)
			os.Exit(1)
		}
		confirm := string(confirmBytes)
		fmt.Println()

		if password != confirm {
			fmt.Println("❌ পাসওয়ার্ড মিলছে না")
			os.Exit(1)
		}
	}

	fmt.Print("🔐 কী জেনারেট হচ্ছে... ")
	keyPair, keyInfo, err := signing.GenerateKeyPair(owner, email, "signing")
	if err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ কী জেনারেট করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	keystorePath := getKeystorePath()
	keyStore, err := signing.NewKeyStore(keystorePath, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কীস্টোর তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if err := keyStore.AddKey(keyPair, keyInfo); err != nil {
		fmt.Fprintf(os.Stderr, "❌ কী সেভ করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("✅ কী সফলভাবে তৈরি হয়েছে!")
	fmt.Printf("🔑 Key ID: %s\n", keyPair.KeyID)
	fmt.Printf("👤 Owner: %s\n", keyInfo.Owner)
	fmt.Printf("🔏 Fingerprint: %s\n", keyInfo.Fingerprint)
	fmt.Printf("📁 Keystore: %s\n", keystorePath)
	fmt.Println("═══════════════════════════════════════════")
}

func cmdList() {
	var password string
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "-password=") {
			password = strings.TrimPrefix(arg, "-password=")
		}
	}
	if password == "" {
		password = promptPassword("🔑 কীস্টোর পাসওয়ার্ড: ")
	}

	keystorePath := getKeystorePath()
	keyStore, err := signing.NewKeyStore(keystorePath, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কীস্টোর খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	keys := keyStore.ListKeys()
	if len(keys) == 0 {
		fmt.Println("📭 কোনো কী নেই")
		fmt.Println("   তৈরি করতে: nilkey generate")
		return
	}

	fmt.Printf("🔑 কীস্টোরে %d টি কী আছে:\n\n", len(keys))
	fmt.Printf("%-20s %-20s %-12s %s\n", "Key ID", "Owner", "Purpose", "Algorithm")
	fmt.Println("────────────────────────────────────────────────────────────")
	for _, key := range keys {
		fmt.Printf("%-20s %-20s %-12s %s\n",
			key.KeyID, key.Owner, key.Purpose, key.Algorithm)
	}
}

func cmdSign() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilkey sign <file.nilax> [-password=pass]")
		os.Exit(1)
	}

	filePath := os.Args[2]
	var password string
	for _, arg := range os.Args[3:] {
		if strings.HasPrefix(arg, "-password=") {
			password = strings.TrimPrefix(arg, "-password=")
		}
	}
	if password == "" {
		password = promptPassword("🔑 কীস্টোর পাসওয়ার্ড: ")
	}

	keystorePath := getKeystorePath()
	keyStore, err := signing.NewKeyStore(keystorePath, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কীস্টোর খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	keys := keyStore.ListKeys()
	if len(keys) == 0 {
		fmt.Println("❌ কোনো কী নেই। প্রথমে তৈরি করুন: nilkey generate")
		os.Exit(1)
	}

	keyPair, keyInfo, err := keyStore.GetKey(keys[0].KeyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কী লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	signer := signing.NewSigner(keyPair, keyInfo)
	signature, err := signer.SignFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ সাইন করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Save signature file alongside the bundle
	sigPath := filePath + ".sig"
	sigJSON, _ := signature.ToJSON()
	os.WriteFile(sigPath, sigJSON, 0644)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("✅ প্যাকেজ সফলভাবে সাইন হয়েছে!")
	fmt.Printf("📦 ফাইল: %s\n", filePath)
	fmt.Printf("🔏 চেকসাম: %s...\n", signature.Checksum[:16])
	fmt.Printf("✍️  সাইনার: %s (%s)\n", signature.SignerName, signature.SignerKeyID)
	fmt.Printf("🔐 অ্যালগরিদম: %s\n", signature.Algorithm)
	fmt.Printf("📄 সিগনেচার ফাইল: %s\n", sigPath)
	fmt.Println("═══════════════════════════════════════════")
}

func cmdVerify() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilkey verify <file.nilax>")
		os.Exit(1)
	}

	filePath := os.Args[2]
	fmt.Printf("🔍 ভেরিফাই করা হচ্ছে: %s\n", filePath)

	sigPath := filePath + ".sig"
	sigBytes, err := os.ReadFile(sigPath)
	if err != nil {
		fmt.Printf("⚠️  সিগনেচার ফাইল (%s) পাওয়া যায়নি।\n", sigPath)
		return
	}

	sig, err := signing.SignatureFromJSON(sigBytes)
	if err != nil {
		fmt.Printf("❌ অবৈধ সিগনেচার ফাইল: %s\n", err)
		return
	}

	fmt.Println("✅ সিগনেচার ভ্যালিড!")
	fmt.Printf("   সাইনার: %s (%s)\n", sig.SignerName, sig.SignerKeyID)
	fmt.Printf("   চেকসাম: %s\n", sig.Checksum)
}

func cmdExport() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilkey export <key-id>")
		os.Exit(1)
	}

	keyID := os.Args[2]
	password := promptPassword("🔑 কীস্টোর পাসওয়ার্ড: ")

	keystorePath := getKeystorePath()
	keyStore, err := signing.NewKeyStore(keystorePath, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কীস্টোর খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	keyPair, _, err := keyStore.GetKey(keyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কী পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("🔑 Public Key (Key ID: %s):\n", keyID)
	fmt.Println(keyPair.GetPublicKeyHex())
	fmt.Println("═══════════════════════════════════════════")
}

func cmdDelete() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilkey delete <key-id>")
		os.Exit(1)
	}

	keyID := os.Args[2]
	password := promptPassword("🔑 কীস্টোর পাসওয়ার্ড: ")

	keystorePath := getKeystorePath()
	keyStore, err := signing.NewKeyStore(keystorePath, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কীস্টোর খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if err := keyStore.DeleteKey(keyID); err != nil {
		fmt.Fprintf(os.Stderr, "❌ কী মুছতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ কী '%s' মুছে ফেলা হয়েছে\n", keyID)
}

func promptPassword(prompt string) string {
	fmt.Print(prompt)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	fmt.Println()
	return string(passwordBytes)
}
