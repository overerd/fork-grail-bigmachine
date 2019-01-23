// Copyright 2018 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package ec2system

import (
	"fmt"
	"path"

	yaml "gopkg.in/yaml.v2"
)

type cloudFile struct {
	Path        string `yaml:"path,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
	Owner       string `yaml:"owner,omitempty"`
	Content     string `yaml:"content,omitempty"`
}

type cloudUnit struct {
	Name    string `yaml:"name,omitempty"`
	Command string `yaml:"command,omitempty"`
	Enable  bool   `yaml:"enable,omitempty"`
	Content string `yaml:"content,omitempty"`

	// Sync determines whether the command should be run synchronously.
	Sync bool `yaml:"-"`
}

// cloudConfig represents a cloud cloud configuration as accepted by
// cloud-init. CloudConfigs can be incrementally defined and then
// rendered by its Marshal method.
type cloudConfig struct {
	// Flavor indicates the flavor of cloud-config; it determines
	// how Systemd units are processed before serialization.
	Flavor Flavor `yaml:"-"`

	WriteFiles []cloudFile `yaml:"write_files,omitempty"`
	CoreOS     struct {
		Update struct {
			RebootStrategy string `yaml:"reboot-strategy,omitempty"`
		} `yaml:"update,omitempty"`
		Units []cloudUnit `yaml:"units,omitempty"`
	} `yaml:"coreos,omitempty"`
	SshAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`

	// RunCmd stores a list of cloud-init run commands.
	RunCmd []string `yaml:"runcmd,omitempty"`
	// Mounts stores a list of cloud-init mounts.
	Mounts [][]string `yaml:"mounts,omitempty"`

	units []cloudUnit
}

// Merge merges cloudConfig d into c. List entries from c are
// appended to d, and key-values are overwritten.
func (c *cloudConfig) Merge(d *cloudConfig) {
	for _, f := range d.WriteFiles {
		c.WriteFiles = append(c.WriteFiles, f)
	}
	if s := d.CoreOS.Update.RebootStrategy; s != "" {
		c.CoreOS.Update.RebootStrategy = s
	}
	for _, u := range d.units {
		c.units = append(c.units, u)
	}
	for _, k := range d.SshAuthorizedKeys {
		c.SshAuthorizedKeys = append(c.SshAuthorizedKeys, k)
	}
}

// AppendFile appends the file f to the cloudConfig c.
func (c *cloudConfig) AppendFile(f cloudFile) {
	c.WriteFiles = append(c.WriteFiles, f)
}

// AppendUnit appends the systemd unit u to the cloudConfig c.
func (c *cloudConfig) AppendUnit(u cloudUnit) {
	c.units = append(c.units, u)
}

// AppendRunCmd appends a run command to the cloud config.
// Note that run commands are only respected in the Ubuntu
// flavor.
func (c *cloudConfig) AppendRunCmd(cmd string) {
	c.RunCmd = append(c.RunCmd, cmd)
}

// AppendMount appends a mount spec. Note that mounts are
// only respected in the Ubuntu flavor.
func (c *cloudConfig) AppendMount(mount []string) {
	c.Mounts = append(c.Mounts, mount)
}

// Marshal renders the cloudConfig into YAML, with the prerequisite
// cloud-config header.
func (c *cloudConfig) Marshal() ([]byte, error) {
	var copy cloudConfig
	copy = *c
	if c.Flavor == CoreOS {
		copy.CoreOS.Units = c.units
	} else {
		if len(c.units) > 0 {
			copy.RunCmd = append(copy.RunCmd, "systemctl daemon-reload")
		}
		for _, u := range c.units {
			if u.Content != "" {
				copy.AppendFile(cloudFile{
					Path:        path.Join("/etc/systemd/system", u.Name),
					Permissions: "0644",
					Content:     u.Content,
				})
			}
			if u.Sync {
				copy.RunCmd = append(copy.RunCmd, fmt.Sprintf("systemctl %s %s", u.Command, u.Name))
			} else {
				copy.RunCmd = append(copy.RunCmd, fmt.Sprintf("systemctl --no-block %s %s", u.Command, u.Name))
			}
		}
	}

	b, err := yaml.Marshal(copy)
	if err != nil {
		return nil, err
	}
	return append([]byte("#cloud-config\n"), b...), nil
}
