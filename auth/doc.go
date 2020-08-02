/*
Package auth is for authentication and authorization. It contains database interfaces (DBGroup, DBUser, DBWorkflow), core types (Group, User, Workflow) and the glue between them.

Workflows

A workflow is a list of groups.
One workflow is assigned (explicitly or inherited) to every node.
One group of the assigned workflow, called "workflow group", is assigned to every version.
Every member of any workflow group can view and edit the node.
Users can revoke and release versions, which means changing its workflow group.
When a member of the last group releases a version, its workflow group becomes "Readers" (id 0) and it is visible to everyone with read permission.

  Example Workflow: economics department, editorial department, image team, editor-in-chief

Different Workflow Models

Workflows could be considered a democratic tool, where one member of every workflow group has to confirm every content change.

Workflows could be designed to represent work steps which have to be done by each group. Versions could be passed back as far as needed, and passed to the next group if a work step is done.

Workflows could be understood in terms of accountability. Workflow groups would be in a subset relation. Any member of the last workflow group could publish on their own.
*/
package auth
