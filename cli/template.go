package cli

// Since Terragrunt is just a thin wrapper for Terraform, and we don't want to repeat every single Terraform command
// in its definition, we don't quite fit into the model of any Go CLI library. Fortunately, urfave/cli allows us to
// override the whole template used for the Usage Text.
const appHelpTemplate = `NAME:
   {{$v := offset .HelpName 6}}{{wrap .HelpName 3}}{{if .Usage}} - {{wrap .Usage $v}}{{end}}

USAGE:
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.HelpName}} <command> [options]{{end}} {{if .Description}}

DESCRIPTION:
   {{wrap .Description 3}}{{end}}{{if .VisibleCommands}}

COMMANDS:{{ $cv := offsetCommands .VisibleCommands 5}}{{range .VisibleCommands}}
   {{$s := .HelpName}}{{$s}}{{ $sp := subtract $cv (offset $s 3) }}{{ indent $sp ""}} {{wrap .Usage $cv}}{{end}}{{end}}

OPTIONS:
   {{range $index, $option := CommmandVisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}{{if not .HideVersion}}

VERSION: {{.Version}}{{if len .Authors}}{{end}}

AUTHOR: {{range .Authors}}{{.}}{{end}} {{end}}
`

const commandHelpTemplate = `NAME:
   {{$v := offset .HelpName 6}}{{wrap .HelpName 3}}{{if .Usage}} - {{wrap .Usage $v}}{{end}}

USAGE:
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}terragrunt {{.HelpName}}{{if .VisibleCommands}} <command>{{end}}{{if .VisibleFlags}} [options]{{end}}{{end}}{{if .Description}}

DESCRIPTION:
   {{wrap .Description 3}}{{end}}{{if .VisibleCommands}}

COMMANDS:{{ $cv := offsetCommands .VisibleCommands 5}}{{range .VisibleCommands}}
   {{$s := .HelpName}}{{$s}}{{ $sp := subtract $cv (offset $s 3) }}{{ indent $sp ""}} {{wrap .Usage $cv}}{{end}}{{end}}{{if .VisibleFlags}}

OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{wrap $option.String 6}}{{end}}{{end}}

`
