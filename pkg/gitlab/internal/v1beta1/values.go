package v1beta1

import (
	_ "embed"

	"context"
	"html/template"
	"strings"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

/* ValueProvider */

func (w *Adapter) Values() support.Values {
	return w.values
}

func (w *Adapter) Hash() string {
	return support.SimpleObjectHash(w.source)
}

/* Helpers */

func (w *Adapter) populate(ctx context.Context) error {
	return support.ChainedOperation{
		w.applyOperatorDefaultValues,
		w.applyUserDefinedValues,
		w.applyOperatorOverrideValues,
		w.applyChartDefaultValues, // it uses coalesce (set value if not present)
	}.Run(ctx)
}

func (w *Adapter) applyUserDefinedValues(_ context.Context) error {
	userValues, err := copystructure.Copy(w.source.Spec.Chart.Values.Object)
	if err != nil {
		return errors.Wrap(err, "failed to clone user-defined values")
	}

	if userValues, ok := userValues.(map[string]interface{}); !ok {
		return errors.New("failed to assert type of user defined values")
	} else if err := w.values.Merge(userValues); err != nil {
		return errors.Wrapf(err, "failed to merge user defined values")
	}

	return nil
}

func (w *Adapter) applyOperatorDefaultValues(_ context.Context) error {
	return w.loadValuesFromTemplate(defaultValuesTemplate, w.templateParameters())
}

func (w *Adapter) applyOperatorOverrideValues(_ context.Context) error {
	params := w.templateParameters()
	params["UseCertManager"] = w.WantsFeature(ConfigureCertManager)

	return w.loadValuesFromTemplate(overrideValuesTemplate, params)
}

func (w *Adapter) loadValuesFromTemplate(template *template.Template, templateParams map[string]interface{}) error {
	var buf = &strings.Builder{}

	if err := template.Execute(buf, templateParams); err != nil {
		return errors.Wrapf(err, "failed to render: %s", template.Name())
	}

	if err := w.values.AddFromYAML(buf.String()); err != nil {
		return errors.Wrapf(err, "can not merge values from: %s", template.Name())
	}

	return nil
}

func (w *Adapter) templateParameters() map[string]interface{} {
	return map[string]interface{}{
		"ReleaseName": w.ReleaseName(),
		"Settings":    appSettings,
	}
}

func (w *Adapter) applyChartDefaultValues(_ context.Context) error {
	charts, err := w.Charts()
	if err != nil {
		return err
	}

	for _, chart := range charts {
		val, err := chartutil.CoalesceValues(chart, w.values)
		if err != nil {
			return errors.Wrapf(err, "failed to coalesce chart values: %s", chart.Name())
		}

		w.values = val.AsMap()
	}

	return nil
}

var appSettings map[string]string

//go:embed default-values.tpl
var defaultValuesSource string
var defaultValuesTemplate *template.Template

//go:embed override-values.tpl
var overrideValuesSource string
var overrideValuesTemplate *template.Template

func init() {
	settings.Load()

	appSettings = map[string]string{
		"AppNonRootServiceAccount": settings.AppNonRootServiceAccount,
		"CertmanagerIssuerEmail":   defaultCertManagerIssuerEmail,
		"ManagerServiceAccount":    settings.ManagerServiceAccount,
		"NginxServiceAccount":      settings.NGINXServiceAccount,
		"PrometheusServiceAccount": settings.PrometheusServiceAccount,
	}

	defaultValuesTemplate = template.Must(template.New("defaultValues").Parse(defaultValuesSource))
	overrideValuesTemplate = template.Must(template.New("overrideValues").Parse(overrideValuesSource))
}
