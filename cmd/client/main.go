/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/megaease/easegress/cmd/client/command"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var exampleUsage = `  # List APIs.
  egctl api list

  # Probe health.
  egctl health

  # List member information.
  egctl member list

  # Purge a easegress member
  egctl member purge <member name>

  # List object kinds.
  egctl object kinds

  # Create an object from a yaml file.
  egctl object create -f <object_spec.yaml>

  # Create an object from stdout.
  cat <object_spec.yaml> | egctl object create

  # Delete an object.
  egctl object delete <object_name>

  # Get an object.
  egctl object get <object_name>

  # List objects.
  egctl object list

  # Update an object from a yaml file.
  egctl object update -f <new_object_spec.yaml>

  # Update an object from stdout.
  cat <new_object_spec.yaml> | egctl object update

  # list objects status
  egctl object status list

  # Get object status
  egctl object status get <object_name>
`

func main() {
	rootCmd := &cobra.Command{
		Use:        "egctl",
		Short:      "A command line admin tool for Easegress.",
		Example:    exampleUsage,
		SuggestFor: []string{"egctl"},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			switch command.CommandlineGlobalFlags.OutputFormat {
			case "yaml", "json":
			default:
				command.ExitWithErrorf("unsupported output format: %s",
					command.CommandlineGlobalFlags.OutputFormat)
			}
		},
	}

	completionCmd := &cobra.Command{
		Use:   "completion bash|zsh",
		Short: "Output shell completion code for the specified shell (bash or zsh)",
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				rootCmd.GenZshCompletion(os.Stdout)
			default:
				command.ExitWithErrorf("unsupported shell %s, expecting bash or zsh", args[0])
			}
		},
		Args: cobra.ExactArgs(1),
	}

	rootCmd.AddCommand(
		command.APICmd(),
		command.HealthCmd(),
		command.ObjectCmd(),
		command.MemberCmd(),
		command.WasmCmd(),
		command.CustomDataKindCmd(),
		command.CustomDataCmd(),
		command.ProfileCmd(),
		completionCmd,
	)

	rootCmd.PersistentFlags().StringVar(&command.CommandlineGlobalFlags.URL,
		"url", "https://localhost:2381", "The address of the Easegress endpoint")
	rootCmd.PersistentFlags().StringVarP(&command.CommandlineGlobalFlags.OutputFormat,
		"output", "o", "yaml", "Output format(json, yaml)")

	err := rootCmd.Execute()
	if err != nil {
		command.ExitWithError(err)
	}
}
