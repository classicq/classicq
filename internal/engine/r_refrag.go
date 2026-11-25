package engine

// r_refrag.c

// node index like entity_t.topnode: leafs past numnodes, 0xffffffff for none
var r_pefragtopnode uint32

var lastlink **efrag_t
var r_emins, r_emaxs vec3_t
var r_addent *entity_t

// Call when removing an object from the world or moving it to another position
func R_RemoveEfrags(ent *entity_t) {
	ef := ent.efrag

	for ef != nil {
		prev := &ef.leaf.efrags
		for {
			walk := *prev
			if walk == nil {
				break
			}
			if walk == ef {
				// remove this fragment
				*prev = ef.leafnext
				break
			} else {
				prev = &walk.leafnext
			}
		}

		old := ef
		ef = ef.entnext

		// put it on the free list
		old.entnext = cl.free_efrags
		cl.free_efrags = old
	}

	ent.efrag = nil
}

func R_SplitEntityOnNode(model *model_t, nodenum uint32) {
	// add an efrag if the node is a leaf

	if nodenum >= uint32(model.numnodes) {
		leafnum := nodenum - uint32(model.numnodes)

		if model.leafsolid[leafnum/32]&(1<<(leafnum%32)) != 0 {
			return
		}

		if r_pefragtopnode == 0xffffffff {
			r_pefragtopnode = nodenum
		}

		leaf := &model.leafs[leafnum]

		// grab an efrag off the free list
		ef := cl.free_efrags
		if ef == nil {
			Com_Printf("Too many efrags!\n")
			return // no free fragments...
		}
		cl.free_efrags = cl.free_efrags.entnext

		ef.entity = r_addent

		// add the entity link
		*lastlink = ef
		lastlink = &ef.entnext
		ef.entnext = nil

		// set the leaf links
		ef.leaf = leaf
		ef.leafnext = leaf.efrags
		leaf.efrags = ef

		return
	}

	// NODE_MIXED

	node := &model.nodes[nodenum]

	splitplane := &model.planes[node.planenum]
	sides := BOX_ON_PLANE_SIDE(r_emins, r_emaxs, splitplane)

	if sides == 3 {
		// split on this plane
		// if this is the first splitter of this bmodel, remember it
		if r_pefragtopnode == 0xffffffff {
			r_pefragtopnode = nodenum
		}
	}

	// recurse down the contacted sides
	if sides&1 != 0 {
		R_SplitEntityOnNode(model, node.childrennum[0])
	}

	if sides&2 != 0 {
		R_SplitEntityOnNode(model, node.childrennum[1])
	}
}

func R_AddEfrags(ent *entity_t) {
	if ent.model == nil {
		return
	}

	r_addent = ent

	lastlink = &ent.efrag
	r_pefragtopnode = 0xffffffff

	entmodel := ent.model

	for i := 0; i < 3; i++ {
		r_emins[i] = ent.origin[i] + entmodel.mins[i]
		r_emaxs[i] = ent.origin[i] + entmodel.maxs[i]
	}

	R_SplitEntityOnNode(cl.worldmodel, 0)

	ent.topnode = r_pefragtopnode
}

func R_StoreEfrags(ppefrag **efrag_t) {
	for pefrag := *ppefrag; pefrag != nil; pefrag = pefrag.leafnext {
		pent := pefrag.entity
		model := pent.model

		if model.modhint == MOD_FLAME && r_drawflame.value == 0 {
			continue
		}

		switch model.type_ {
		case mod_alias, mod_brush, mod_sprite:
			if pent.visframe != r_framecount {
				CL_AddEntity(pent)
				pent.visframe = r_framecount // mark that we've recorded this entity for this frame
			}

		default:
			Sys_Error("R_StoreEfrags: Bad entity type %d", model.type_)
		}
	}
}
