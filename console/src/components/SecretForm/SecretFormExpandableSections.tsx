import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardBody, CardTitle, ExpandableSection, FormGroup } from '@patternfly/react-core';
import { SecretDefinition, SecretField, SecretFormData } from '../../types';
import { getSecretFieldKind } from '../../vaultSecrets';
import { GenerateField } from './GenerateField';
import { FileField } from './FileField';
import { IniField } from './IniField';
import { StaticField } from './StaticField';
import './SecretForm.css';

export type SecretFormExpandableSectionsProps = {
  secrets: SecretDefinition[];
  secretFormData: SecretFormData;
  expandedSections: Record<string, boolean>;
  onToggleSection: (sectionName: string) => void;
  onFieldChange: (secretName: string, fieldName: string, value: string | null) => void;
  /** When true, empty `file` / `ini` fields show validation errors (after a blocked submit). */
  secretsValidationAttempted?: boolean;
};

/**
 * Expandable cards per secret definition with typed fields (generate, file, ini, static).
 */
export function SecretFormExpandableSections({
  secrets,
  secretFormData,
  expandedSections,
  onToggleSection,
  onFieldChange,
  secretsValidationAttempted = false,
}: SecretFormExpandableSectionsProps) {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');

  const renderField = (
    secret: SecretDefinition,
    field: SecretField,
    uploadFieldError?: string,
  ) => {
    const fieldType = getSecretFieldKind(field);
    const value = secretFormData[secret.name]?.[field.name] || '';

    const commonProps = {
      field,
      value,
      onChange: (newValue: string | null) => onFieldChange(secret.name, field.name, newValue),
    };

    switch (fieldType) {
      case 'generate':
        return <GenerateField key={field.name} {...commonProps} />;
      case 'file':
        return <FileField key={field.name} {...commonProps} fieldError={uploadFieldError} />;
      case 'ini':
        return <IniField key={field.name} {...commonProps} fieldError={uploadFieldError} />;
      case 'static':
        return <StaticField key={field.name} {...commonProps} />;
      default:
        return <StaticField key={field.name} {...commonProps} />;
    }
  };

  return (
    <div className="patterns-operator__secret-form">
      {secrets.map((secret) => (
        <ExpandableSection
          key={secret.name}
          toggleText={secret.name}
          isExpanded={expandedSections[secret.name] || false}
          onToggle={() => onToggleSection(secret.name)}
          className="patterns-operator__secret-section"
        >
          <Card>
            <CardTitle>{secret.name}</CardTitle>
            <CardBody>
              {secret.fields.map((field) => {
                const fieldType = getSecretFieldKind(field);
                const rawVal = secretFormData[secret.name]?.[field.name];
                const emptyFileIni =
                  (fieldType === 'file' || fieldType === 'ini') &&
                  (rawVal === null || rawVal === undefined || rawVal === '');
                const showFileIniError = secretsValidationAttempted && emptyFileIni;
                const helperInvalid =
                  fieldType === 'file'
                    ? t('A file upload is required.')
                    : t('An INI file upload is required.');
                return (
                  <FormGroup
                    key={field.name}
                    label={field.name}
                    isRequired={true}
                    fieldId={`secret-${secret.name}-${field.name}`}
                    className="patterns-operator__secret-field"
                  >
                    {renderField(secret, field, showFileIniError ? helperInvalid : undefined)}
                  </FormGroup>
                );
              })}
            </CardBody>
          </Card>
        </ExpandableSection>
      ))}
    </div>
  );
}
