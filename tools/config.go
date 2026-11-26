/*
 * Copyright (C) 2020-2022, IrineSistiana
 *
 * This file is part of mosdns.
 *
 * mosdns is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mosdns is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newGenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen config.json",
		Short: "Generate a template JSON configuration.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !strings.EqualFold(filepath.Ext(args[0]), ".json") {
				return fmt.Errorf("configuration must be a .json file")
			}
			return os.WriteFile(args[0], []byte(configTemplate), 0o644)
		},
		DisableFlagsInUseLine: true,
	}
}

const configTemplate = `{
  "log": {
    "level": "info"
  },
  "plugins": [
    {
      "tag": "forward_google",
      "type": "forward",
      "args": {
        "upstreams": [{"addr": "https://8.8.8.8/dns-query"}]
      }
    },
    {
      "tag": "udp_server",
      "type": "udp_server",
      "args": {"entry": "forward_google", "listen": "127.0.0.1:53"}
    },
    {
      "tag": "tcp_server",
      "type": "tcp_server",
      "args": {"entry": "forward_google", "listen": "127.0.0.1:53"}
    }
  ]
}
`
