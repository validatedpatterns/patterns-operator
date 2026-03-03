import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useRouteMatch } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Button,
  Card,
  CardBody,
  CardTitle,
  ExpandableSection,
  Form,
  FormGroup,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { fetchPattern, fetchSecretTemplate } from '../api';
import { Pattern, SecretTemplate, SecretFormData } from '../types';
import { GenerateField } from './SecretForm/GenerateField';
import { PromptField } from './SecretForm/PromptField';
import { FileField } from './SecretForm/FileField';
import { IniField } from './SecretForm/IniField';
import { StaticField } from './SecretForm/StaticField';
import './SecretForm/SecretForm.css';

export default function SecretFormPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const match = useRouteMatch<{ name: string }>('/patterns/install/:name/secrets');
  const name = match?.params?.name;

  const [loading, setLoading] = React.useState(true);
  const [fetchError, setFetchError] = React.useState<string | null>(null);
  const [submitting, setSubmitting] = React.useState(false);

  const [pattern, setPattern] = React.useState<Pattern | null>(null);
  const [secretTemplate, setSecretTemplate] = React.useState<SecretTemplate | null>(null);
  const [secretFormData, setSecretFormData] = React.useState<SecretFormData>({});

  React.useEffect(() => {
    Promise.all([fetchPattern(name), fetchSecretTemplate(name)])
      .then(([patternData, templateData]) => {
        setPattern(patternData);
        setSecretTemplate(templateData);

        // Initialize form data
        if (templateData) {
          const initialData: SecretFormData = {};
          templateData.secrets.forEach((secret) => {
            initialData[secret.name] = {};
            secret.fields.forEach((field) => {
              initialData[secret.name][field.name] = '';
            });
          });
          setSecretFormData(initialData);
        }

        setLoading(false);
      })
      .catch((err) => {
        setFetchError(err?.message || String(err));
        setLoading(false);
      });
  }, [name]);

  const handleFieldChange = (
    secretName: string,
    fieldName: string,
    value: string | File | null,
  ) => {
    setSecretFormData((prev) => ({
      ...prev,
      [secretName]: {
        ...prev[secretName],
        [fieldName]: value,
      },
    }));
  };

  const handleContinue = () => {
    setSubmitting(true);
    // Navigate back to install page with secret data in state
    history.push(`/patterns/install/${name}`, { secretData: secretFormData, secretTemplate });
  };

  const handleSkip = () => {
    // Navigate back to install page without secrets
    history.push(`/patterns/install/${name}`);
  };

  const getFieldType = (field: SecretField): 'generate' | 'prompt' | 'file' | 'ini' | 'static' => {
    if (field.onMissingValue === 'generate') return 'generate';
    if (field.path) return 'file';
    if (field.ini_file) return 'ini';
    if (field.value !== undefined && field.value !== null) return 'static';
    return 'prompt'; // Default to prompt for required fields
  };

  const renderField = (secret: SecretDefinition, field: SecretField) => {
    const fieldType = getFieldType(field);
    const value = secretFormData[secret.name]?.[field.name] || '';

    const commonProps = {
      field,
      value,
      onChange: (newValue: string | File | null) =>
        handleFieldChange(secret.name, field.name, newValue),
    };

    switch (fieldType) {
      case 'generate':
        return <GenerateField key={field.name} {...commonProps} />;
      case 'prompt':
        return <PromptField key={field.name} {...commonProps} />;
      case 'file':
        return <FileField key={field.name} {...commonProps} />;
      case 'ini':
        return <IniField key={field.name} {...commonProps} />;
      case 'static':
        return <StaticField key={field.name} {...commonProps} />;
      default:
        return <PromptField key={field.name} {...commonProps} />;
    }
  };

  if (loading) {
    return (
      <PageSection>
        <Spinner aria-label={t('Loading secret template')} />
      </PageSection>
    );
  }

  if (fetchError) {
    return (
      <PageSection>
        <Alert variant="danger" title={t('Failed to load secret template')}>
          {fetchError}
        </Alert>
      </PageSection>
    );
  }

  if (!secretTemplate) {
    // No secret template found, redirect back to install
    React.useEffect(() => {
      history.push(`/patterns/install/${name}`);
    }, []);
    return null;
  }

  return (
    <>
      <Helmet>
        <title>{t('Configure Secrets')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">
          {t('Configure Secrets for {{patternName}}', {
            patternName: pattern?.display_name || name,
          })}
        </Title>
      </PageSection>
      <PageSection>
        <Form className="patterns-operator__secret-form">
          {secretTemplate.secrets.map((secret, index) => (
            <ExpandableSection
              key={secret.name}
              toggleText={secret.name}
              isExpanded={index === 0} // First section expanded by default
              className="patterns-operator__secret-section"
            >
              <Card>
                <CardTitle>{secret.name}</CardTitle>
                <CardBody>
                  {secret.fields.map((field) => (
                    <FormGroup
                      key={field.name}
                      label={field.name}
                      helperText={field.description}
                      isRequired={getFieldType(field) === 'prompt'}
                      fieldId={`secret-${secret.name}-${field.name}`}
                      className="patterns-operator__secret-field"
                    >
                      {renderField(secret, field)}
                    </FormGroup>
                  ))}
                </CardBody>
              </Card>
            </ExpandableSection>
          ))}

          <ActionGroup className="patterns-operator__secret-actions">
            <Button
              variant="primary"
              onClick={handleContinue}
              isLoading={submitting}
              isDisabled={submitting}
            >
              {t('Continue to Install')}
            </Button>
            <Button variant="secondary" onClick={handleSkip}>
              {t('Skip Secrets')}
            </Button>
            <Button variant="link" onClick={() => history.push('/patterns')}>
              {t('Cancel')}
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
}
