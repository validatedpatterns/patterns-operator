import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  FileUpload,
  FormHelperText,
  FormSelect,
  FormSelectOption,
  HelperText,
  HelperTextItem,
  TextArea,
} from '@patternfly/react-core';

interface IniFieldProps {
  field: {
    name: string;
    description?: string;
    ini_file?: string;
    ini_section?: string;
    ini_key?: string;
  };
  value: string;
  onChange: (value: string) => void;
}

interface ParsedIni {
  [section: string]: {
    [key: string]: string;
  };
}

export const IniField: React.FC<IniFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [filename, setFilename] = React.useState('');
  const [fileContent, setFileContent] = React.useState('');
  const [parsedIni, setParsedIni] = React.useState<ParsedIni | null>(null);
  const [selectedSection, setSelectedSection] = React.useState('');
  const [parseError, setParseError] = React.useState<string | null>(null);

  const parseIniContent = (content: string): ParsedIni => {
    const result: ParsedIni = {};
    let currentSection = '';

    const lines = content.split('\n');

    for (const line of lines) {
      const trimmed = line.trim();

      // Skip empty lines and comments
      if (!trimmed || trimmed.startsWith('#') || trimmed.startsWith(';')) {
        continue;
      }

      // Section header
      if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
        currentSection = trimmed.slice(1, -1).trim();
        result[currentSection] = {};
        continue;
      }

      // Key-value pair
      const equalIndex = trimmed.indexOf('=');
      if (equalIndex > 0) {
        const key = trimmed.slice(0, equalIndex).trim();
        const val = trimmed.slice(equalIndex + 1).trim();

        if (!currentSection) {
          // Create a default section for keys without section
          if (!result['default']) {
            result['default'] = {};
          }
          result['default'][key] = val;
        } else {
          result[currentSection][key] = val;
        }
      }
    }

    return result;
  };

  const handleFileInputChange = (file: File | null, filename: string) => {
    setFilename(filename);

    if (!file) {
      setFileContent('');
      setParsedIni(null);
      setSelectedSection('');
      setParseError(null);
      onChange('');
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      try {
        const content = reader.result as string;
        setFileContent(content);

        const parsed = parseIniContent(content);
        setParsedIni(parsed);
        setParseError(null);

        // Auto-select section if specified in template
        if (field.ini_section && parsed[field.ini_section]) {
          setSelectedSection(field.ini_section);
          updateValue(parsed, field.ini_section);
        } else {
          // Auto-select first section
          const firstSection = Object.keys(parsed)[0];
          if (firstSection) {
            setSelectedSection(firstSection);
            updateValue(parsed, firstSection);
          }
        }
      } catch (error) {
        setParseError(error?.message || t('Failed to parse INI file'));
        setParsedIni(null);
        onChange('');
      }
    };

    reader.onerror = () => {
      setParseError(t('Failed to read file'));
      setParsedIni(null);
      onChange('');
    };

    reader.readAsText(file);
  };

  const updateValue = (parsed: ParsedIni, section: string) => {
    if (field.ini_key && parsed[section] && parsed[section][field.ini_key]) {
      onChange(parsed[section][field.ini_key]);
    } else {
      // If no specific key, return the whole section as key=value pairs
      const sectionData = parsed[section];
      const formatted = Object.entries(sectionData)
        .map(([key, val]) => `${key}=${val}`)
        .join('\n');
      onChange(formatted);
    }
  };

  const handleSectionChange = (_event: React.FormEvent<HTMLSelectElement>, section: string) => {
    setSelectedSection(section);
    if (parsedIni) {
      updateValue(parsedIni, section);
    }
  };

  const handleClear = () => {
    setFilename('');
    setFileContent('');
    setParsedIni(null);
    setSelectedSection('');
    setParseError(null);
    onChange('');
  };

  return (
    <>
      <FileUpload
        id={`ini-file-${field.name}`}
        value={fileContent}
        filename={filename}
        onChange={handleFileInputChange}
        onClearClick={handleClear}
        dropzoneProps={{
          accept: {
            'text/*': ['.ini', '.conf', '.config', '.credentials'],
          },
        }}
      />

      {parseError && (
        <Alert variant="danger" title={t('Parse Error')} isInline>
          {parseError}
        </Alert>
      )}

      {parsedIni && Object.keys(parsedIni).length > 1 && (
        <FormSelect
          value={selectedSection}
          onChange={handleSectionChange}
          aria-label={t('Select INI section')}
        >
          <FormSelectOption value="" label={t('Select a section')} />
          {Object.keys(parsedIni).map((section) => (
            <FormSelectOption key={section} value={section} label={section} />
          ))}
        </FormSelect>
      )}

      {value && (
        <TextArea value={value} isReadOnly aria-label={t('Extracted value preview')} rows={4} />
      )}

      <FormHelperText>
        <HelperText>
          <HelperTextItem>
            {field.description || t('Upload an INI/configuration file to extract values')}
          </HelperTextItem>
          {field.ini_section && (
            <HelperTextItem>
              {t('Target section: {{section}}', { section: field.ini_section })}
            </HelperTextItem>
          )}
          {field.ini_key && (
            <HelperTextItem>{t('Target key: {{key}}', { key: field.ini_key })}</HelperTextItem>
          )}
        </HelperText>
      </FormHelperText>
    </>
  );
};
