// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package main

import (
	"os"
	"path/filepath"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thanos-io/thanos/pkg/errutil"
	"github.com/thanos-io/thanos/pkg/extkingpin"
	"github.com/thanos-io/thanos/pkg/rules"
)

func registerTools(app *extkingpin.App) {
	cmd := app.Command("tools", "Tools utility commands")

	registerBucket(cmd)
	registerCheckRules(cmd)
}

func registerCheckRules(app extkingpin.AppClause) {
	cmd := app.Command("rules-check", "Check if the rule files are valid or not.")
	ruleFiles := cmd.Flag("rules", "The rule files glob to check (repeated).").Required().Strings()

	cmd.Setup(func(g *run.Group, logger log.Logger, reg *prometheus.Registry, _ opentracing.Tracer, _ <-chan struct{}, _ bool) error {
		// Dummy actor to immediately kill the group after the run function returns.
		g.Add(func() error { return nil }, func(error) {})
		return checkRulesFiles(logger, ruleFiles)
	})
}

func checkRulesFiles(logger log.Logger, patterns *[]string) error {
	var failed errutil.MultiError

	for _, p := range *patterns {
		level.Info(logger).Log("msg", "checking", "pattern", p)
		matches, err := filepath.Glob(p)
		if err != nil || matches == nil {
			err = errors.New("matching file not found")
			level.Error(logger).Log("result", "FAILED", "error", err)
			level.Info(logger).Log()
			failed.Add(err)
			continue
		}
		for _, fn := range matches {
			level.Info(logger).Log("msg", "checking", "filename", fn)
			f, er := os.Open(fn)
			if er != nil {
				level.Error(logger).Log("result", "FAILED", "error", er)
				level.Info(logger).Log()
				failed.Add(err)
				continue
			}
			defer func() { _ = f.Close() }()

			n, errs := rules.ValidateAndCount(f)
			if errs.Err() != nil {
				level.Error(logger).Log("result", "FAILED")
				for _, e := range errs {
					level.Error(logger).Log("error", e.Error())
					failed.Add(e)
				}
				level.Info(logger).Log()
				continue
			}
			level.Info(logger).Log("result", "SUCCESS", "rules found", n)
		}
	}
	return failed.Err()
}
