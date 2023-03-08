import * as React from 'react';
import { Button, Modal, ModalVariant } from '@patternfly/react-core';
import '../main.css';

export default function PatternsCatalogModel(props) {
  // TODO: Find a better way to check/validate that data comes in
  if (props.data.metadata === undefined) {
    return null;
  }

  const data = props.data.spec

  return (
    <>
      <Modal
        actions={[
          <Button key="confirm" variant="primary">
            Deploy Pattern
          </Button>,
        ]}
        isOpen={props.isOpen}
        onClose={props.onClose}
        title={data.pattern.name}
        variant={ModalVariant.medium}
      >
        <h2>Organization</h2>
        <p><b>Name:</b> {data.organization.name}</p>
        <p><b>Description:</b> {data.organization.description}</p>
        <p><b>URL:</b> <a href={data.organization.url}>{data.organization.url}</a></p>
        <p><b>Maintainers:</b> {data.organization.maintainers}</p>
        <br /> {/* TODO: Replace this hack */}
        <h2>Pattern</h2>
        <p><b>Name:</b> {data.pattern.name}</p>
        <p><b>Description:</b> {data.pattern.longDescription}</p>
        <p><b>Branch:</b> {data.pattern.branch}</p>
        <p><b>Type:</b> {data.pattern.badge}</p>
        <p><b>URL:</b> <a href={data.pattern.url}>{data.pattern.url}</a></p>
      </Modal>
    </>
  );
}
