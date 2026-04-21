import * as React from 'react';
import { Card, CardBody, CardTitle, ExpandableSection, FormGroup } from '@patternfly/react-core';
import { SecretDefinition, SecretField, SecretFormData } from '../../types';
import { GenerateField } from './GenerateField';
import { PromptField } from './PromptField';
import { FileField } from './FileField';
import { IniField } from './IniField';
import { StaticField } from './StaticField';
import './SecretForm.css';

function getFieldType(field: SecretField): 'generate' | 'prompt' | 'file' | 'ini' | 'static' {
  if (field.onMissingValue === 'generate') return 'generate';
  if (field.path) return 'file';
  if (field.ini_file) return 'ini';
  if (field.value !== undefined && field.value !== null) return 'static';
  return 'prompt';
}

export type SecretFormExpandableSectionsProps = {
  secrets: SecretDefinition[];
  secretFormData: SecretFormData;
  expandedSections: Record<string, boolean>;
  onToggleSection: (sectionName: string) => void;
  onFieldChange: (secretName: string, fieldName: string, value: string | File | null) => void;
};

/**
 * Expandable cards per secret definition with typed fields (generate, prompt, file, ini, static).
 */
export function SecretFormExpandableSections({
  secrets,
  secretFormData,
  expandedSections,
  onToggleSection,
  onFieldChange,
}: SecretFormExpandableSectionsProps) {
  const renderField = (secret: SecretDefinition, field: SecretField) => {
    const fieldType = getFieldType(field);
    const value = secretFormData[secret.name]?.[field.name] || '';

    const commonProps = {
      field,
      value,
      onChange: (newValue: string | File | null) =>
        onFieldChange(secret.name, field.name, newValue),
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
    </div>
  );
}
