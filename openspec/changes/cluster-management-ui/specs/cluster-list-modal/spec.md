# cluster-list-modal

## ADDED Requirements

### Requirement: ClusterView SHALL display the cluster list by default

The cluster list view loads without showing the add form first.

#### Scenario: Page loads with list visible
- **WHEN** the user navigates to the clusters view
- **THEN** the cluster list is displayed immediately, and the add form is not visible

---

### Requirement: ClusterView SHALL open a modal dialog when creating a new cluster

Clicking the "新建集群" button opens a modal containing the add form.

#### Scenario: User clicks "新建集群" button
- **WHEN** the user clicks the "新建集群" button in the toolbar
- **THEN** a modal dialog opens containing name and kubeconfig input fields

#### Scenario: Modal closes on cancel
- **WHEN** the user clicks "取消" or the overlay behind the modal
- **THEN** the modal closes and the form is not submitted

---

### Requirement: ClusterView SHALL close the modal and refresh the list after successful cluster creation

Submitting the form in the modal closes it and updates the cluster list.

#### Scenario: Successful form submission closes modal
- **WHEN** the user fills in name and kubeconfig and clicks "添加"
- **THEN** the API call is made, the modal closes, and the cluster list refreshes to show the new entry
